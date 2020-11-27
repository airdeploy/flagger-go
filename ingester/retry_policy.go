package ingester

import (
	"errors"
	"github.com/airdeploy/flagger-go/v3/log"
)

const defaultMaxMemorySize = 2e8 // 100 MB

func newRetryPolicy() *retryPolicy {
	return &retryPolicy{
		maxMemorySizeInBytes: defaultMaxMemorySize,
	}
}

// this method will trigger ingest with data and ingestionURL
// if it fails to do so, retryPolicy remembers the data(if there is enough space)
// and will try again at the next ingest call.
// If the next call of ingest doesn't return error then retryPolicy tries to send remembered data
// in the queue order
func (rt *retryPolicy) ingest(request *retryPolicyRequest) {
	//add one httpRequest to the wait group
	err := request.httpRequest(request.data, request.ingestionURL)
	if err != nil {
		rt.putToQueue(request.data, request.callback)
	} else {
		// server is up
		request.callback(nil)
		rt.releaseWait(request.ingestionURL, request.httpRequest)
	}
}

func (rt *retryPolicy) putToQueue(data []byte, callback RetryPolicyCallback) {
	if rt.currentMemorySize+size(data) < rt.maxMemorySizeInBytes {
		rt.addToQueue(data, callback)
	} else {
		if size(data) > rt.maxMemorySizeInBytes {
			log.Warnf("Ingester: data is too large, size: %d, max size: %d", size(data), rt.maxMemorySizeInBytes)
			return
		}
		// removes first element from queue until there is enough space to add new data chunk
		for {
			if rt.currentMemorySize+size(data) < rt.maxMemorySizeInBytes {
				rt.addToQueue(data, nil)
				break
			}

			first := rt.shift()
			// notify about data will never be sent
			first.callback(errors.New("queue is full, first element removed"))
		}
	}
}

// removes first element from queue
// Caution: must be called when queue.length > 0
func (rt *retryPolicy) shift() *queueElement {
	first := rt.queue[0]
	rt.queue = rt.queue[1:]
	rt.currentMemorySize -= size(first.data)
	return first
}

func (rt *retryPolicy) addToQueue(data []byte, callback RetryPolicyCallback) {
	rt.queue = append(rt.queue, &queueElement{
		data:     data,
		callback: callback,
	})
	rt.currentMemorySize += size(data)
}

func size(data []byte) int64 {
	return int64(24 + len(data))
}

func (rt *retryPolicy) releaseWait(ingestionURL string, callback httpRequestType) {
	for {
		// quit if queue is empty
		if len(rt.queue) == 0 {
			return
		}
		// take first element
		first := rt.queue[0]
		// try to send it
		err := callback(first.data, ingestionURL)
		if err != nil {
			// can't release anything
			return
		}
		// success. Removes fist element
		first = rt.shift()
		// notify about success
		first.callback(nil)
	}
}

// SetMaxSize set maximum seize for the buffer
// not thread safe
func (rt *retryPolicy) SetMaxSize(maxMemorySizeInBytes int64) {
	rt.maxMemorySizeInBytes = maxMemorySizeInBytes
}
