package extpool

import (
	"container/list"
	"errors"
	"sync"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/network/payload"
	"github.com/ethereum/go-ethereum/common"
)

// Ledger is enough of Blockchain to satisfy Pool.
type Ledger interface {
	BlockHeight() uint32
	IsExtensibleAllowed(common.Address) bool
	VerifyWitness(common.Address, hash.Hashable, *transaction.Witness) error
}

// Pool represents pool of extensible payloads.
type Pool struct {
	lock     sync.RWMutex
	verified map[common.Hash]*list.Element
	senders  map[common.Address]*list.List
	// singleCap represents maximum number of payloads from the single sender.
	singleCap int
	chain     Ledger
}

// New returns new payload pool using provided chain.
func New(bc Ledger, capacity int) *Pool {
	if capacity <= 0 {
		panic("invalid capacity")
	}

	return &Pool{
		verified:  make(map[common.Hash]*list.Element),
		senders:   make(map[common.Address]*list.List),
		singleCap: capacity,
		chain:     bc,
	}
}

var (
	errDisallowedSender = errors.New("disallowed sender")
	errInvalidHeight    = errors.New("invalid height")
)

// Add adds extensible payload to the pool.
// First return value specifies if payload was new.
// Second one is nil if and only if payload is valid.
func (p *Pool) Add(e *payload.Extensible) (bool, error) {
	if ok, err := p.verify(e); err != nil || !ok {
		return ok, err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	h := e.Hash()
	if _, ok := p.verified[h]; ok {
		return false, nil
	}

	lst, ok := p.senders[e.Sender]
	if ok && lst.Len() >= p.singleCap {
		value := lst.Remove(lst.Front())
		delete(p.verified, value.(*payload.Extensible).Hash())
	} else if !ok {
		lst = list.New()
		p.senders[e.Sender] = lst
	}
	p.verified[h] = lst.PushBack(e)
	return true, nil
}

func (p *Pool) verify(e *payload.Extensible) (bool, error) {
	if err := p.chain.VerifyWitness(e.Sender, e, &e.Witness); err != nil {
		return false, err
	}
	h := p.chain.BlockHeight()
	if h < e.ValidBlockStart || e.ValidBlockEnd <= h {
		// We can receive consensus payload for the last or next block
		// which leads to unwanted node disconnect.
		if e.ValidBlockEnd == h {
			return false, nil
		}
		return false, errInvalidHeight
	}
	if !p.chain.IsExtensibleAllowed(e.Sender) {
		return false, errDisallowedSender
	}
	return true, nil
}

// Get returns payload by hash.
func (p *Pool) Get(h common.Hash) *payload.Extensible {
	p.lock.RLock()
	defer p.lock.RUnlock()

	elem, ok := p.verified[h]
	if !ok {
		return nil
	}
	payload, ok := elem.Value.(*payload.Extensible)
	if !ok {
		return nil
	}
	return payload
}

const extensibleVerifyMaxGAS = 6000000

// RemoveStale removes invalid payloads after block processing.
func (p *Pool) RemoveStale(index uint32) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for s, lst := range p.senders {
		for elem := lst.Front(); elem != nil; {
			e := elem.Value.(*payload.Extensible)
			h := e.Hash()
			old := elem
			elem = elem.Next()

			if e.ValidBlockEnd <= index || !p.chain.IsExtensibleAllowed(e.Sender) {
				delete(p.verified, h)
				lst.Remove(old)
				continue
			}
			if err := p.chain.VerifyWitness(e.Sender, e, &e.Witness); err != nil {
				delete(p.verified, h)
				lst.Remove(old)
				continue
			}
		}
		if lst.Len() == 0 {
			delete(p.senders, s)
		}
	}
}
