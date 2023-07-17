package native

import (
	"encoding/binary"
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativeids"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativenames"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/ethereum/go-ethereum/common"
)

const (
	DefaultFeePerByte uint64 = 1000
	DefaultGasPrice   uint64 = 10000000000 //10GWei

	PrefixFeePerByte    byte = 0x01
	PrefixGasPrice      byte = 0x08
	PrefixBlockedAcount byte = 0x11
)

var (
	PolicyAddress      = common.Address(common.BytesToAddress([]byte{nativeids.Policy}))
	ErrAccountBlocked  = errors.New("account blocked")
	ErrContractBlocked = errors.New("contract blocked")
)

type Policy struct {
	state.NativeContract
	cs    *Contracts
	cache *policyCache
}

type policyCache struct {
	feePerByte atomic.Value
	gasPrice   atomic.Value
}

func NewPolicy(cs *Contracts) *Policy {
	p := &Policy{
		NativeContract: state.NativeContract{
			Name: nativenames.Policy,
			Contract: state.Contract{
				Address:  PolicyAddress,
				CodeHash: hash.Keccak256(PolicyAddress[:]),
				Code:     PolicyAddress[:],
			},
		},
		cs:    cs,
		cache: &policyCache{},
	}
	policyAbi, contractCalls, err := constructAbi(p)
	if err != nil {
		panic(err)
	}
	p.Abi = *policyAbi
	p.ContractCalls = contractCalls
	return p
}

func (p *Policy) createBlockKey(address common.Address) []byte {
	return makeAddressKey(PrefixBlockedAcount, address)
}

func (p *Policy) UpdateCache(d *dao.Simple) error {
	p.cache.feePerByte.Store(p.GetFeePerByteFromStorage(d))
	p.cache.gasPrice.Store(p.GetGasPriceFromStorage(d))
	return nil
}

func (p *Policy) PostPersist(d *dao.Simple, _ *block.Block) error {
	p.UpdateCache(d)
	return nil
}

func (p *Policy) Initialize(d *dao.Simple) error {
	item := make([]byte, 8)

	binary.BigEndian.PutUint64(item, DefaultGasPrice)
	d.PutStorageItem(p.Address, []byte{PrefixGasPrice}, item)

	binary.BigEndian.PutUint64(item, DefaultFeePerByte)
	d.PutStorageItem(p.Address, []byte{PrefixFeePerByte}, item)

	return nil
}

func (p *Policy) ContractCall_setFeePerByte(ic InteropContext, fee uint64) error {
	err := p.cs.Designate.checkConsensus(ic)
	if err != nil {
		return err
	}
	item := make([]byte, 8)
	binary.BigEndian.PutUint64(item, fee)
	ic.Dao().PutStorageItem(p.Address, []byte{PrefixFeePerByte}, item)
	log(ic, p.Address, item, p.Abi.Events["setFeePerByte"].ID)
	return nil
}

func (p *Policy) ContractCall_setGasPrice(ic InteropContext, gasPrice uint64) error {
	err := p.cs.Designate.checkConsensus(ic)
	if err != nil {
		return err
	}
	item := make([]byte, 8)
	binary.BigEndian.PutUint64(item, gasPrice)
	ic.Dao().PutStorageItem(p.Address, []byte{PrefixGasPrice}, item)
	log(ic, p.Address, item, p.Abi.Events["setGasPrice"].ID)
	return nil
}

func (p *Policy) ContractCall_blockAccount(ic InteropContext, address common.Address) error {
	err := p.cs.Designate.checkConsensus(ic)
	if err != nil {
		return err
	}
	if err := p.checkSystem(ic, address); err != nil {
		return err
	}
	key := p.createBlockKey(address)
	item := ic.Dao().GetStorageItem(p.Address, key)
	if item != nil {
		return errors.New("already blocked")
	}
	ic.Dao().PutStorageItem(p.Address, key, []byte{1})
	log(ic, p.Address, nil, p.Abi.Events["blockAccount"].ID, common.BytesToHash(address[:]))
	return nil
}

func (p *Policy) ContractCall_unblockAccount(ic InteropContext, address common.Address) error {
	err := p.cs.Designate.checkConsensus(ic)
	if err != nil {
		return err
	}
	if err := p.checkSystem(ic, address); err != nil {
		return err
	}
	key := p.createBlockKey(address)
	item := ic.Dao().GetStorageItem(p.Address, key)
	if item == nil {
		return errors.New("account isn't blocked")
	}
	ic.Dao().DeleteStorageItem(p.Address, key)
	log(ic, p.Address, nil, p.Abi.Events["unblockAccount"].ID, common.BytesToHash(address[:]))
	return nil
}

func (p *Policy) GetFeePerByte(s *dao.Simple) uint64 {
	val := p.cache.feePerByte.Load()
	if val != nil {
		return val.(uint64)
	}
	return p.GetFeePerByteFromStorage(s)
}

func (p *Policy) GetFeePerByteFromStorage(s *dao.Simple) uint64 {
	item := s.GetStorageItem(p.Address, []byte{PrefixFeePerByte})
	if item == nil {
		return DefaultFeePerByte
	}
	return binary.BigEndian.Uint64(item)
}

func (p *Policy) GetGasPrice(s *dao.Simple) *big.Int {
	val := p.cache.gasPrice.Load()
	if val != nil {
		return val.(*big.Int)
	}
	return p.GetGasPriceFromStorage(s)
}

func (p *Policy) GetGasPriceFromStorage(s *dao.Simple) *big.Int {
	item := s.GetStorageItem(p.Address, []byte{PrefixGasPrice})
	if item == nil {
		return big.NewInt(int64(DefaultGasPrice))
	}
	return big.NewInt(int64(binary.BigEndian.Uint64(item)))
}

func (p *Policy) checkSystem(ic InteropContext, address common.Address) error {
	if ic.PersistingBlock() == nil {
		return ErrNoBlock
	}
	addrs, err := p.cs.Designate.GetSysAddresses(ic.Dao(), ic.PersistingBlock().Index)
	if err != nil {
		return err
	}
	for _, account := range addrs {
		if address == account {
			return errors.New("system account")
		}
	}
	return nil
}

func (p *Policy) IsBlocked(d *dao.Simple, address common.Address) bool {
	key := p.createBlockKey(address)
	item := d.GetStorageItem(p.Address, key)
	return item != nil
}

func (p *Policy) RequiredGas(ic InteropContext, input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := p.Abi.MethodById(input[:4])
	if err != nil {
		return 0
	}
	switch method.Name {
	case "initialize":
		return 0
	case "setFeePerByte", "setGasPrice":
		return defaultNativeWriteFee
	default:
		return 0
	}
}

func (p *Policy) Run(ic InteropContext, input []byte) ([]byte, error) {
	return contractCall(p, &p.NativeContract, ic, input)
}
