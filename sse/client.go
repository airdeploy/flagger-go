package sse

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/pkg/errors"
)

// check public interface on compile time
var _ interface {
	SetURL(u string)
} = new(Client)

// CallBack represent callback that process new Flagger configuration
type CallBack func(v *core.Configuration)

// NewClient return the new instance SSE.Client
func NewClient(cb CallBack) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		changeURL: make(chan string, 32),
		rt: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: -1, // disable keep-alive timeout
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		cb:                cb,
		reconnectInterval: 30 * time.Second,
		keepaliveTimeout:  30 * time.Second,
		ctx:               ctx,
		cancel:            cancel,
		addDelayBefore:    1 * time.Minute,
	}
}

func newClientWithRT(cb CallBack, rt http.RoundTripper) *Client {
	client := NewClient(cb)
	client.rt = rt
	return client
}

// Client represent SSE client
type Client struct {
	changeURL chan string
	rt        http.RoundTripper
	cb        CallBack
	one       sync.Once
	ctx       context.Context
	cancel    context.CancelFunc

	reconnectInterval time.Duration
	keepaliveTimeout  time.Duration
	addDelayBefore    time.Duration
}

// SetURL using to changing subscribing url
func (c *Client) SetURL(URL string) {
	c.changeURL <- URL
	c.one.Do(func() { go c.infiniteLoop() })
}

// Shutdown - closes sse connection to free up the resources
func (c *Client) Shutdown() {
	c.cancel()
}

// foundation of sse client:
// for {
// 		reconnect(onSuccessConnected func() {
//			go messagesReader(dataChannel)
//			for {
//				select {
//				case <-dataChannel
//					continue
//				case keepalive timeout expired
//					return
//				case handle change URL
//					return
//				}
//			}
// 		})
//		sleep between reconnected
// }
func (c *Client) infiniteLoop() {
	rand.Seed(time.Now().UnixNano())
	URL := <-c.changeURL

	// infinite reconnection
	for {
		isURLHasChanged := false // used to skipped reconnecting interval

		var connectedAt time.Time
		// this function try to connect to server by given URL, on success - called given callback
		c.reconnect(URL, func(r io.Reader) {
			// on connected callback scope
			connectedAt = time.Now()

			dataChannel := make(chan [][]byte, 32)

			// this goroutine sends messages from http connection to channel
			// goroutine can be aborted by closing the response body or interrupting the http connection
			go func() {
				defer close(dataChannel)
				messagesReader(r, dataChannel) // (1) producer
			}()

			// event loop, handle:
			//  - receives next message from connection
			//  - keep alive timeout
			//  - changes server URL
			keepAliveTimer := time.NewTimer(c.keepaliveTimeout)
			defer keepAliveTimer.Stop()
			for {
				select {
				case message, ok := <-dataChannel: // (1) consumer
					if !ok {
						log.Debugf("SSE: connection is closed")
						return // messagesReader goroutine was returned
					}

					keepAliveTimer.Reset(c.keepaliveTimeout)
					processMessage(message, c.cb)

				case <-keepAliveTimer.C:
					log.Debugf("SSE: keepAlive timeout has expired, timeout: %s", c.keepaliveTimeout)
					return

				case u := <-c.changeURL:
					URL = u
					isURLHasChanged = true
					log.Debugf("SSE: URL has changed to %s", URL)
					return
				case <-c.ctx.Done():
					return
				}
			}
		})

		log.Debugf("SSE: not accepting new messages")

		if /* NOT */ !isURLHasChanged {
			reconnectWithDelay := time.Since(connectedAt) < c.addDelayBefore

			var interval time.Duration
			if reconnectWithDelay {
				// reconnect interval random in range [0, reconnectInterval]
				interval = time.Duration(rand.Int63n(int64(c.reconnectInterval)))
			} else {
				interval = 0
			}

			log.Debugf("SSE: Waiting %s to reconnect", time.Duration.Round(interval, time.Millisecond))
			// server URL can be changed during reconnection timeout, so:
			timer := time.NewTimer(interval)
			select {
			case u := <-c.changeURL:
				timer.Stop()
				URL = u
				log.Debugf("SSE: URL has changed during reconnection phase to %s", URL)

			case <-timer.C:
				timer.Stop()
				log.Debugf("SSE: reconnect interval has passed, reconnecting to %+v", URL)

			case <-c.ctx.Done():
				log.Debugf("SSE: shut down")
				return
			}
		}
	}
}

func (c *Client) reconnect(URL string, onConnected func(r io.Reader)) {
	req, err := http.NewRequest(http.MethodGet, URL, http.NoBody)
	if err != nil {
		log.Debugf("SSE: error when connecting to URL: %+v", URL)
		return
	}

	req.Header.Set("accept", "text/name-stream")
	req.Header.Set("accept-encoding", "gzip")
	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		log.Debugf("SSE: error %+v when connecting to URL: %+v", err.Error(), URL)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Debugf("SSE: connection failed, status: \"%s\", code: \"%d\"", resp.Status, resp.StatusCode)
		return
	}

	log.Debugf("SSE: connected to %s", URL)
	switch resp.Header.Get("content-encoding") {
	case "gzip":
		r, err := gzip.NewReader(resp.Body)
		if err != nil {
			log.Debugf("SSE: failed to read gzipped data, url: %+v", URL)
			return
		}
		onConnected(r)

	default:
		onConnected(resp.Body)
	}
}

// messagesReader - read lines from reader, groups lines by an empty line from reader
func messagesReader(r io.Reader, dataChannel chan<- [][]byte) {
	scanner := bufio.NewScanner(r)
	message := make([][]byte, 0, 4)
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			dataChannel <- message
			message = make([][]byte, 0, 4)
			continue
		}
		message = append(message, []byte(text))
	}
}

func processMessage(message [][]byte, cb CallBack) {
	_, kind, data, err := parseMessage(message)
	if err != nil {
		log.Warnf("SSE: parse message error: %+v", err)
		return
	}

	if kind == "flagConfigUpdate" {
		log.Debugf("SSE: has received the message: %s", message)
		var v *core.Configuration
		err := json.Unmarshal(data, &v)
		if err != nil {
			log.Warnf("SSE: json parse error: %+v, data: %+v", err, string(data))
			return
		}

		cb(v)
	}
}

func parseMessage(message [][]byte) (id, kind string, data []byte, _ error) {
	if len(message) != 3 {
		return "", "", nil, errors.Errorf("expect 3 lines")
	}

	if len(message[0]) < 3 {
		return "", "", nil, errors.Errorf("expect id:...")
	}

	if len(message[1]) < 6 {
		return "", "", nil, errors.Errorf("expect event:...")
	}

	if len(message[2]) < 5 {
		return "", "", nil, errors.Errorf("expect data:...")
	}

	id = strings.TrimSpace(string(message[0][3:]))
	kind = strings.TrimSpace(string(message[1][6:]))
	data = bytes.TrimSpace(message[2][5:])

	return id, kind, data, nil
}
