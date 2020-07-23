package sse

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/airdeploy/flagger-go/core"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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
	processMessage([][]byte{
		[]byte(`id:74f24698-ef4a-4a67-bc5a-2d583ed54a0a`),
		[]byte(`event:flagConfigUpdate`),
		[]byte(`data:{"hashKey":"F87CD00E55F6E5997676A8B771F1335D"}`),
	}, func(v *core.Configuration) {
		assert.Equal(t, &core.Configuration{HashKey: "F87CD00E55F6E5997676A8B771F1335D"}, v)
	})

	processMessage([][]byte{
		[]byte(`id:74f24698-ef4a-4a67-bc5a-2d583ed54a0a`),
		[]byte(`event:keepalive`),
		[]byte(`data:`),
	}, func(v *core.Configuration) {
		assert.Fail(t, "expect no call")
	})
}

func Test_SSE_Connection(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})
	ctx := context.Background()
	flaggerConfigMessage := getConfigMessage()
	broker := NewSSEServer(ctx, flaggerConfigMessage)
	go func() {
		serve := http.ListenAndServe("localhost:3100", broker)
		log.Fatal("HTTP server error: ", serve)
	}()

	t.Run("Client test", func(t *testing.T) {

		count := 0
		time.Sleep(100 * time.Millisecond)

		cli := NewClient(func(v *core.Configuration) {
			logrus.Debugf("callback: %+v", v)
			count++
		})
		cli.SetURL("http://localhost:3100")
		//wait to connect
		time.Sleep(100 * time.Millisecond)

		broker.Notifier <- flaggerConfigMessage
		broker.Notifier <- flaggerConfigMessage
		broker.Notifier <- flaggerConfigMessage
		broker.Notifier <- flaggerConfigMessage

		//wait to deliver
		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, 5, count)

		cli.Shutdown()
	})

	t.Run("Shutdown stops new configuration from being pushed from the server", func(t *testing.T) {
		count := 0
		time.Sleep(100 * time.Millisecond)

		sseClient := NewClient(func(v *core.Configuration) {
			logrus.Debugf("callback: %+v", v)
			count++
		})
		sseClient.SetURL("http://localhost:3100")

		broker.Notifier <- flaggerConfigMessage
		//wait to deliver
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, count)

		sseClient.Shutdown()

		broker.Notifier <- flaggerConfigMessage
		broker.Notifier <- flaggerConfigMessage
		broker.Notifier <- flaggerConfigMessage
		broker.Notifier <- flaggerConfigMessage
		//wait to deliver, but no delivery should happened
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, count)

	})

	t.Run("Shutdown stops SSE from reconnection", func(t *testing.T) {

		count := 0
		time.Sleep(100 * time.Millisecond)
		assert.Zero(t, len(broker.clients))

		sseClient := NewClient(func(v *core.Configuration) {
			logrus.Debugf("callback: %+v", v)
			count++
		})
		sseClient.SetURL("http://localhost:3100")

		time.Sleep(10 * time.Millisecond) // wait to connect
		assert.Equal(t, 1, len(broker.clients))

		sseClient.Shutdown()

		time.Sleep(100 * time.Millisecond)
		assert.Zero(t, len(broker.clients))

		time.Sleep(100 * time.Millisecond)
	})
}

func Test_ClientSlow(t *testing.T) {
	t.SkipNow()
	ctx := context.Background()

	flaggerConfigMessage := getConfigMessage()
	broker := NewSSEServer(ctx, flaggerConfigMessage)
	go func() {
		log.Fatal("HTTP server error: ", http.ListenAndServe("localhost:3100", broker))
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

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})
	time.Sleep(1 * time.Second)

	cli := NewClient(func(v *core.Configuration) {
		logrus.Debugf("callback: %+v", v)
	})
	cli.SetURL("http://localhost:3100")

	time.Sleep(10 * time.Minute)
}

func getConfigMessage() []byte {
	flaggerConfigMessage := []byte("id: 76c58618-b75c-4872-a107-986998601fe4\n" +
		"event: flagConfigUpdate\n" +
		"data: ")
	config, _ := ioutil.ReadFile("../configuration.json")

	config = bytes.Replace(config, []byte(" "), []byte(""), -1)
	config = bytes.Replace(config, []byte("\n"), []byte(""), -1)
	flaggerConfigMessage = append(flaggerConfigMessage, config...)
	flaggerConfigMessage = append(flaggerConfigMessage, []byte("\n\n")...)
	return flaggerConfigMessage
}
