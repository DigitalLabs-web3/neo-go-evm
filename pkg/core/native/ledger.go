package native

import (
	"encoding/binary"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativeids"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativenames"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/ethereum/go-ethereum/common"
)

const (
	prefixNonce = 0x01
)

var (
	LedgerAddress common.Address = common.Address(common.BytesToAddress([]byte{nativeids.Ledger}))
)

type Ledger struct {
	state.NativeContract
}

func createNonceKey(a common.Address) []byte {
	return makeAddressKey(prefixNonce, a)
}

func NewLedger() *Ledger {
	return &Ledger{
		NativeContract: state.NativeContract{
			Name: nativenames.Ledger,
			Contract: state.Contract{
				Address: LedgerAddress,
				Code:    []byte{},
			},
		},
	}
}

func (l *Ledger) GetNonce(s *dao.Simple, h common.Address) uint64 {
	item := s.GetStorageItem(l.Address, createNonceKey(h))
	if item == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(item)
}

func (l *Ledger) SetNonce(s *dao.Simple, h common.Address, nonce uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, nonce)
	s.PutStorageItem(l.Address, createNonceKey(h), b)
}
