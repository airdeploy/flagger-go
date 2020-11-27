package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/airdeploy/flagger-go/v3/log"
)

// Broker is core struct of sse server
type Broker struct {
	// events are pushed to this channel by the main events-gathering routine
	Notifier chan []byte

	// New client connections
	newClients chan chan []byte

	// Closed client connections
	closingClients chan chan []byte

	// Client connections registry
	clients map[chan []byte]bool

	messageOnJoin []byte

	newClientConnectHandler func()
}

// NewSSEServer creates new sse server intance
func NewSSEServer(ctx context.Context, messageOnJoin []byte) *Broker {
	// Instantiate a broker
	broker := &Broker{
		Notifier:       make(chan []byte, 1),
		newClients:     make(chan chan []byte),
		closingClients: make(chan chan []byte),
		clients:        make(map[chan []byte]bool),
		messageOnJoin:  messageOnJoin,
	}

	// Set it running - listening and broadcasting events
	go broker.listen(ctx)
	return broker
}

// SetNewClientConnectHandler sets a handler  that will be called when new client connects
func (broker *Broker) SetNewClientConnectHandler(handler func()) {
	broker.newClientConnectHandler = handler
}

// ServeHTTP
func (broker *Broker) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	// Make sure that the writer supports flushing.
	//
	flusher, ok := rw.(http.Flusher)

	if !ok {
		http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	rw.Header().Set("Access-Control-Allow-Origin", "*")

	// Each connection registers its own message channel with the Broker's connections registry
	messageChan := make(chan []byte)

	// Signal the broker that we have a new connection
	broker.newClients <- messageChan

	// Remove this client from the map of connected clients
	// when this handler exits.
	defer func() {
		broker.closingClients <- messageChan
	}()

	// Listen to connection close and un-register messageChan
	// notify := rw.(http.CloseNotifier).CloseNotify()
	notify := req.Context().Done()

	go func() {
		<-notify
		broker.closingClients <- messageChan
	}()

	for {

		// Write to the ResponseWriter
		// Server Sent events compatible
		_, _ = fmt.Fprintf(rw, "%s", <-messageChan)

		// Flush the data immediatly instead of buffering it for later.
		flusher.Flush()
	}

}

func (broker *Broker) listen(ctx context.Context) {
	for {
		select {
		case s := <-broker.newClients:
			if broker.newClientConnectHandler != nil {
				broker.newClientConnectHandler()
			}

			// A new client has connected.
			// Register their message channel
			broker.clients[s] = true

			s <- broker.messageOnJoin
			log.Debugf("SERVER: Client added. %d registered clients", len(broker.clients))
		case s := <-broker.closingClients:

			// A client has dettached and we want to
			// stop sending them messages.
			delete(broker.clients, s)
			log.Debugf("SERVER: Removed client. %d registered clients", len(broker.clients))
		case event := <-broker.Notifier:

			// We got a new event from the outside!
			// Send event to all connected clients
			for clientMessageChan := range broker.clients {
				clientMessageChan <- event
			}
		case <-ctx.Done():
			log.Debugf("SERVER: ctx.Done()")
			return
		}
	}

}

// ClientsCount returns the amount of connected clients
func (broker *Broker) ClientsCount() int {
	return len(broker.clients)
}
