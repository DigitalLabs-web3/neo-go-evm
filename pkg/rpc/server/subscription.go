package server

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/request"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gorilla/websocket"
	"go.uber.org/atomic"
)

type (
	// subscriber is an event subscriber.
	subscriber struct {
		writer    chan<- *websocket.PreparedMessage
		ws        *websocket.Conn
		overflown atomic.Bool
		// These work like slots as there is not a lot of them (it's
		// cheaper doing it this way rather than creating a map),
		// pointing to EventID is an obvious overkill at the moment, but
		// that's not for long.
		feeds [maxFeeds]feed
	}
	feed struct {
		event  response.EventID
		filter interface{}
	}
)

const (
	// Maximum number of subscriptions per one client.
	maxFeeds = 16

	// This sets notification messages buffer depth, it may seem to be quite
	// big, but there is a big gap in speed between internal event processing
	// and networking communication that is combined with spiky nature of our
	// event generation process, which leads to lots of events generated in
	// short time and they will put some pressure to this buffer (consider
	// ~500 invocation txs in one block with some notifications). At the same
	// time this channel is about sending pointers, so it's doesn't cost
	// a lot in terms of memory used.
	notificationBufSize = 1024
)

func (f *feed) Matches(r *response.Notification) bool {
	if r.Event != f.event {
		return false
	}
	if f.filter == nil {
		return true
	}
	switch f.event {
	case response.BlockEventID:
		filt := f.filter.(request.BlockFilter)
		b := r.Payload[0].(*block.Block)
		return int(b.PrimaryIndex) == filt.Primary
	case response.TransactionEventID:
		filt := f.filter.(request.TxFilter)
		tx := r.Payload[0].(*transaction.Transaction)
		sender := tx.From()
		senderOK := filt.Sender == nil || sender == (*filt.Sender)
		return senderOK
	case response.NotificationEventID:
		filt := f.filter.(request.NotificationFilter)
		notification := r.Payload[0].(*types.Log)
		hashOk := filt.Contract == nil || notification.Address == (*filt.Contract)
		return hashOk
	case response.ExecutionEventID:
		filt := f.filter.(request.ExecutionFilter)
		applog := r.Payload[0].(*types.Receipt)
		return applog.Status == filt.State
	}
	return false
}
