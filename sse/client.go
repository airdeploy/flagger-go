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
	}
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

		// this function try to connect to server by given URL, on success - called given callback
		c.reconnect(URL, func(r io.Reader) {
			// on connected callback scope

			dataChannel := make(chan [][]byte, 32)

			// this goroutine just produce messages from connection into given channel
			// goroutine can be returned by two ways: close response body or interrupt connection by server
			go func() {
				defer close(dataChannel)
				messagesReader(r, dataChannel) // (1) producer
			}()

			// event loop, handle:
			//  - receive next message from connection
			//  - keep alive timeout
			//  - change server URL
			keepAliveTimer := time.NewTimer(c.keepaliveTimeout)
			defer keepAliveTimer.Stop()
			for {
				select {
				case message, ok := <-dataChannel: // (1) consumer
					if !ok {
						log.Debugf("SSE connection is closed")
						return // messagesReader goroutine was returned
					}

					keepAliveTimer.Reset(c.keepaliveTimeout)

					log.Debugf("SSE: receive message: %s", message)
					processMessage(message, c.cb)

				case <-keepAliveTimer.C:
					log.Debugf("SSE: connection was expired, keep-alive timeout: %s", c.keepaliveTimeout)
					return

				case u := <-c.changeURL:
					URL = u
					isURLHasChanged = true
					log.Debugf("SSE: changing URL: %s", URL)
					return
				case <-c.ctx.Done():
					return
				}
			}
		})

		if /* NOT */ !isURLHasChanged {
			// reconnect interval: [reconnectInterval, 2*reconnectInterval]
			interval := c.reconnectInterval + time.Duration(rand.Int63n(int64(c.reconnectInterval)))

			// server URL can be changed during reconnection timeout, so:
			timer := time.NewTimer(interval)
			select {
			case u := <-c.changeURL:
				timer.Stop()
				URL = u
				log.Debugf("SSE url has change to %s", URL)

			case <-timer.C:
				timer.Stop()
			case <-c.ctx.Done():
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
		log.Debugf("SSE: error when connecting to URL: %+v", URL)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Debugf("SSE connection failed, status: \"%s\", code: \"%d\"", resp.Status, resp.StatusCode)
		return
	}

	log.Debugf("SSE: connected to %s", URL)
	switch resp.Header.Get("content-encoding") {
	case "gzip":
		r, err := gzip.NewReader(resp.Body)
		if err != nil {
			log.Debugf("SSE: reconnect: %+v", errors.Wrap(err, "gzip.NewReader"))
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
		log.Warnf("SSE: parseMessage: %+v", errors.WithStack(err))
		return
	}

	if kind == "flagConfigUpdate" {
		var v *core.Configuration
		err := json.Unmarshal(data, &v)
		if err != nil {
			log.Warnf("SSE: parse json: %+v", errors.WithStack(err))
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

	if len(message[1]) < 5 {
		return "", "", nil, errors.Errorf("expect data:...")
	}

	id = strings.TrimSpace(string(message[0][3:]))
	kind = strings.TrimSpace(string(message[1][6:]))
	data = bytes.TrimSpace(message[2][5:])

	return id, kind, data, nil
}
