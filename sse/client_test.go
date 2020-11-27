package sse

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"github.com/airdeploy/flagger-go/v3/internal"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/h2non/gock.v1"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/stretchr/testify/assert"
)

const (
	defaultConfigPath = "../testdata/configuration.json"
	ssePort           = "3100"
	sseURL            = "http://localhost:" + ssePort
	notCalledMessage  = "not expected to be called"
)

func Test_messageReader(t *testing.T) {
	buf := make([]byte, 0, 515)
	buf = append(buf, []byte(`id:74f24698-ef4a-4a67-bc5a-2d583ed54a0a`+"\n")...)
	buf = append(buf, []byte(`event:flagConfigUpdate`+"\n")...)
	buf = append(buf, []byte(`data:{"hashKey":"E22C2F9EA2DD92E08140EFE1FD446477"}`+"\n")...)
	buf = append(buf, []byte("\n")...)
	buf = append(buf, []byte(`id:5113e9b8-707d-4667-8a3a-46a624ce9c98`+"\n")...)
	buf = append(buf, []byte(`event:keepalive`+"\n")...)
	buf = append(buf, []byte(`data:`+"\n")...)
	buf = append(buf, []byte("\n")...)
	buf = append(buf, []byte("\n")...)
	buf = append(buf, []byte(`id:a1f1c8e4-f3ab-49d6-a9e9-27ce4a405707`+"\n")...)
	buf = append(buf, []byte(`event:keepalive`+"\n")...)
	buf = append(buf, []byte(`data:`+"\n")...)
	buf = append(buf, []byte("\n")...)

	buffer := bytes.NewBuffer(buf)
	dataChannel := make(chan [][]byte, 32)

	messagesReader(buffer, dataChannel)
	close(dataChannel)

	assert.EqualValues(t, [][]byte{
		[]byte(`id:74f24698-ef4a-4a67-bc5a-2d583ed54a0a`),
		[]byte(`event:flagConfigUpdate`),
		[]byte(`data:{"hashKey":"E22C2F9EA2DD92E08140EFE1FD446477"}`),
	}, <-dataChannel)

	assert.EqualValues(t, [][]byte{
		[]byte(`id:5113e9b8-707d-4667-8a3a-46a624ce9c98`),
		[]byte(`event:keepalive`),
		[]byte(`data:`),
	}, <-dataChannel)

	assert.EqualValues(t, [][]byte{}, <-dataChannel)

	assert.EqualValues(t, [][]byte{
		[]byte(`id:a1f1c8e4-f3ab-49d6-a9e9-27ce4a405707`),
		[]byte(`event:keepalive`),
		[]byte(`data:`),
	}, <-dataChannel)
}

func Test_processMessage(t *testing.T) {
	t.Run("cb called with correct config", func(t *testing.T) {
		processMessage([][]byte{
			[]byte(`id:74f24698-ef4a-4a67-bc5a-2d583ed54a0a`),
			[]byte(`event:flagConfigUpdate`),
			[]byte(`data:{"hashKey":"F87CD00E55F6E5997676A8B771F1335D"}`),
		}, func(v *core.Configuration) {
			assert.Equal(t, &core.Configuration{HashKey: "F87CD00E55F6E5997676A8B771F1335D"}, v)
		})
	})

	t.Run("cb is not called", func(t *testing.T) {
		t.Run("because event is keepalive", func(t *testing.T) {
			processMessage([][]byte{
				[]byte(`id:74f24698-ef4a-4a67-bc5a-2d583ed54a0a`),
				[]byte(`event:keepalive`),
				[]byte(`data:`),
			}, func(v *core.Configuration) {
				assert.Fail(t, notCalledMessage)
			})
		})

		t.Run("because error occurred during parsing the message", func(t *testing.T) {
			processMessage([][]byte{}, func(v *core.Configuration) {
				assert.Fail(t, notCalledMessage)
			})
		})
		t.Run("because error occurred during parsing the config", func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			processMessage([][]byte{
				[]byte(`id:74f24698-ef4a-4a67-bc5a-2d583ed54a0a`),
				[]byte(`event:flagConfigUpdate`),
				[]byte(`data: not A json`),
			}, func(v *core.Configuration) {
				assert.Fail(t, notCalledMessage)
			})
		})
	})

}

func Test_parseMessage(t *testing.T) {
	t.Run("message must be 3 lines long", func(t *testing.T) {
		_, _, _, err := parseMessage([][]byte{[]byte("id:id")})
		assert.NotNil(t, err)
		assert.Equal(t, "expect 3 lines", err.Error())
	})

	t.Run("message must have an id", func(t *testing.T) {
		_, _, _, err := parseMessage([][]byte{
			[]byte(""),
			[]byte("event:"),
			[]byte("data:data"),
		})
		assert.NotNil(t, err)
		assert.Equal(t, "expect id:...", err.Error())
	})

	t.Run("message must have an event", func(t *testing.T) {
		_, _, _, err := parseMessage([][]byte{
			[]byte("id:id"),
			[]byte(""),
			[]byte("data:data"),
		})
		assert.NotNil(t, err)
		assert.Equal(t, "expect event:...", err.Error())
	})

	t.Run("message must have data", func(t *testing.T) {
		_, _, _, err := parseMessage([][]byte{
			[]byte("id:id"),
			[]byte("event:event"),
			[]byte(""),
		})
		assert.NotNil(t, err)
		assert.Equal(t, "expect data:...", err.Error())
	})

	t.Run("successfully parse a message", func(t *testing.T) {
		id, event, data, err := parseMessage([][]byte{
			[]byte("id:id"),
			[]byte("event:event"),
			[]byte("data:data"),
		})
		assert.Nil(t, err)
		assert.Equal(t, "id", id)
		assert.Equal(t, "event", event)
		assert.Equal(t, []byte("data"), data)
	})

}

func TestClient_reconnect(t *testing.T) {

	t.Run("failed scenarios", func(t *testing.T) {
		sseClient := NewClient(func(v *core.Configuration) {
			assert.Fail(t, notCalledMessage)
		})

		t.Run("Error parsing URL", func(t *testing.T) {
			sseClient.reconnect("http://invalidurl6934%^&*#$GR$I#F", func(r io.Reader) {
				assert.Fail(t, notCalledMessage)
			})
		})

		t.Run("Cannot connect to URL", func(t *testing.T) {
			sseClient.reconnect("http://invalidurl6934:9432/sdfs", func(r io.Reader) {
				assert.Fail(t, notCalledMessage)
			})
		})

		t.Run("http status != 200", func(t *testing.T) {
			gock.New("http://sse").
				Get("/test").
				Reply(http.StatusBadRequest)

			sseClient = newClientWithRT(func(v *core.Configuration) {
				assert.Fail(t, notCalledMessage)
			}, gock.DefaultTransport)
			sseClient.reconnect("http://sse/test", func(r io.Reader) {
				assert.Fail(t, notCalledMessage)
			})
		})

		t.Run("failed to parse gzip", func(t *testing.T) {
			gock.New("http://sse").
				Get("/test").
				Reply(http.StatusOK).
				AddHeader("content-encoding", "gzip")

			sseClient = newClientWithRT(func(v *core.Configuration) {
				assert.Fail(t, notCalledMessage)
			}, gock.DefaultTransport)
			sseClient.reconnect("http://sse/test", func(r io.Reader) {
				assert.Fail(t, notCalledMessage)
			})
		})
	})

	t.Run("send gzipped data", func(t *testing.T) {
		data := []byte("data")

		var compressed bytes.Buffer
		w := gzip.NewWriter(&compressed)
		if _, err := w.Write(data); err != nil {
			assert.Fail(t, notCalledMessage)
		}
		if err := w.Close(); err != nil {
			assert.Fail(t, notCalledMessage)
		}

		gock.New("http://sse").
			Get("/test").
			Reply(http.StatusOK).
			Body(&compressed).
			AddHeader("content-encoding", "gzip")
		sseClient := newClientWithRT(func(v *core.Configuration) {
			assert.Fail(t, notCalledMessage)
		}, gock.DefaultTransport)
		sseClient.reconnect("http://sse/test", func(r io.Reader) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				text := scanner.Text()
				assert.Equal(t, string(data), text)
			}
		})
	})
}

func Test_SSE_Connection(t *testing.T) {
	ctx := context.Background()
	flaggerConfigMessage := getConfigMessage()
	sseServer := internal.NewSSEServer(ctx, flaggerConfigMessage)
	go func() {
		serve := http.ListenAndServe("localhost:"+ssePort, sseServer)
		log.Fatal("HTTP server error: ", serve)
	}()
	timeToConnect := 10 * time.Millisecond

	// wait for the server to start
	time.Sleep(100 * time.Millisecond)

	t.Run("Client must receive config 5 times", func(t *testing.T) {
		count := 0

		sseClient := NewClient(func(v *core.Configuration) {
			count++
		})
		sseClient.SetURL(sseURL)
		// wait to connect
		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 4; i++ {
			sseServer.Notifier <- flaggerConfigMessage
		}

		//wait to deliver
		time.Sleep(100 * time.Millisecond)

		// message on connect and 4 more
		assert.Equal(t, 5, count)

		sseClient.Shutdown()

		// wait for the disconnect
		time.Sleep(100 * time.Millisecond)
		assert.Zero(t, sseServer.ClientsCount())
	})

	t.Run("Shutdown stops new configuration from being pushed from the server", func(t *testing.T) {
		count := 0

		sseClient := NewClient(func(v *core.Configuration) {
			count++
		})
		sseClient.SetURL(sseURL)

		sseServer.Notifier <- flaggerConfigMessage
		//wait to deliver
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, count)

		sseClient.Shutdown()

		sseServer.Notifier <- flaggerConfigMessage
		sseServer.Notifier <- flaggerConfigMessage
		sseServer.Notifier <- flaggerConfigMessage
		sseServer.Notifier <- flaggerConfigMessage
		//wait to deliver, but no delivery should happened
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, count)

	})

	t.Run("Shutdown stops SSE from reconnection", func(t *testing.T) {
		count := 0
		assert.Zero(t, sseServer.ClientsCount())

		sseClient := NewClient(func(v *core.Configuration) {
			count++
		})
		sseClient.setReconnectInterval(10 * time.Millisecond)
		sseClient.SetURL(sseURL)

		time.Sleep(timeToConnect)
		assert.Equal(t, 1, sseServer.ClientsCount())

		sseClient.Shutdown()

		time.Sleep(100 * time.Millisecond)
		assert.Zero(t, sseServer.ClientsCount())
	})

	t.Run("keepAlive expires, sseClient reconnects", func(t *testing.T) {
		sseClient := NewClient(func(_ *core.Configuration) {})

		keepAliveTimeout := time.Millisecond * 500
		sseClient.setKeepaliveTimeout(keepAliveTimeout)
		reconnectInterval := time.Millisecond * 100
		sseClient.setReconnectInterval(reconnectInterval)

		sseClient.SetURL(sseURL)
		// wait to connect
		time.Sleep(timeToConnect)
		assert.Equal(t, 1, sseServer.ClientsCount())

		// keep alive timeout
		time.Sleep(keepAliveTimeout - timeToConnect + 10*time.Millisecond)
		assert.Equal(t, 0, sseServer.ClientsCount())

		// wait to reconnect
		time.Sleep(reconnectInterval * 2)
		assert.Equal(t, 1, sseServer.ClientsCount())

		sseClient.Shutdown()
		time.Sleep(timeToConnect)
	})

	t.Run("Changing URL triggers reconnect", func(t *testing.T) {
		count := 0
		sseServer.SetNewClientConnectHandler(func() {
			count++
		})
		sseClient := NewClient(func(v *core.Configuration) {})

		times := 3

		for i := 0; i < times; i++ {
			sseClient.SetURL(sseURL)
			time.Sleep(timeToConnect)
			assert.Equal(t, 1, sseServer.ClientsCount())
		}

		assert.Equal(t, count, times)

		sseClient.Shutdown()
		time.Sleep(timeToConnect)

		sseServer.SetNewClientConnectHandler(nil)
	})

	t.Run("url is changed during reconnection interval, triggers immediate reconnection", func(t *testing.T) {
		count := 0
		sseServer.SetNewClientConnectHandler(func() {
			count++
		})
		sseClient := NewClient(func(_ *core.Configuration) {})

		keepAliveTimeout := 200 * time.Millisecond
		sseClient.setKeepaliveTimeout(keepAliveTimeout)

		sseClient.SetURL(sseURL)

		// wait for keep alive timeout
		time.Sleep(keepAliveTimeout + 50*time.Millisecond)
		assert.Zero(t, sseServer.ClientsCount())

		sseClient.SetURL(sseURL)
		time.Sleep(timeToConnect)

		assert.Equal(t, 1, sseServer.ClientsCount())

		sseClient.Shutdown()
		time.Sleep(timeToConnect)
		assert.Zero(t, sseServer.ClientsCount())
		sseServer.SetNewClientConnectHandler(nil)
	})

	t.Run("shutdown is called during reconnection interval", func(t *testing.T) {
		sseClient := NewClient(func(_ *core.Configuration) {})

		keepAliveTimeout := 200 * time.Millisecond
		sseClient.setKeepaliveTimeout(keepAliveTimeout)

		sseClient.SetURL(sseURL)

		sseClient.Shutdown()

		time.Sleep(timeToConnect)
		assert.Zero(t, sseServer.ClientsCount())
	})
}

func Test_ClientSlow(t *testing.T) {
	t.SkipNow()
	ctx := context.Background()

	flaggerConfigMessage := getConfigMessage()
	broker := internal.NewSSEServer(ctx, flaggerConfigMessage)
	go func() {
		log.Fatal("HTTP server error: ", http.ListenAndServe("localhost:"+ssePort, broker))
	}()

	keepalive := []byte("id: 37237fc0-dee2-44cc-be21-806fe0e65201\n" +
		"event: keepalive\n" +
		"data:\n\n")

	go func() {
		for {
			time.Sleep(time.Second * 35)
			broker.Notifier <- keepalive
		}
	}()

	go func() {
		for {
			time.Sleep(time.Second * 45)
			broker.Notifier <- flaggerConfigMessage
		}
	}()

	time.Sleep(1 * time.Second)

	cli := NewClient(func(v *core.Configuration) {})
	cli.SetURL(sseURL)

	time.Sleep(10 * time.Minute)
}

func getConfigMessage() []byte {
	flaggerConfigMessage := []byte("id: " + uuid.New().String() + "\n" +
		"event: flagConfigUpdate\n" +
		"data: ")
	config, _ := ioutil.ReadFile(defaultConfigPath)

	config = bytes.Replace(config, []byte(" "), []byte(""), -1)
	config = bytes.Replace(config, []byte("\n"), []byte(""), -1)
	flaggerConfigMessage = append(flaggerConfigMessage, config...)
	flaggerConfigMessage = append(flaggerConfigMessage, []byte("\n\n")...)
	return flaggerConfigMessage
}
