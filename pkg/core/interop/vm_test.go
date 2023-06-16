package interop

import (
	"math/big"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativenames"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/statedb"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

type testNativeContracts struct {
	cs *native.Contracts
}

func newTestNativeContracts() *testNativeContracts {
	return &testNativeContracts{
		cs: native.NewContracts(config.ProtocolConfiguration{
			InitialGASSupply: 100,
		}),
	}
}

func (t *testNativeContracts) Contracts() *native.Contracts {
	return t.cs
}

type testContractRef struct {
	Addr common.Address
}

func (t testContractRef) Address() common.Address {
	return t.Addr
}

func TestNativeContract(t *testing.T) {
	cs := newTestNativeContracts()
	ms := storage.NewMemoryStore()
	mc := storage.NewMemCachedStore(ms)
	d := dao.NewSimple(mc)
	sd := statedb.NewStateDB(d, cs)
	vm := NewEVM(vm.BlockContext{
		BlockNumber: big.NewInt(1),
		CanTransfer: func(vm.StateDB, common.Address, *big.Int) bool { return true },
		Transfer:    func(vm.StateDB, common.Address, common.Address, *big.Int) {},
	}, vm.TxContext{}, sd, config.ProtocolConfiguration{}, nil, nil)
	data := []byte{0x00}
	ret, left, err := vm.Call(testContractRef{Addr: common.BytesToAddress([]byte{})}, common.Address(cs.Contracts().Designate.Address), data, 0, big.NewInt(0))
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), left)
	assert.Equal(t, []byte(nativenames.Designation), ret)
}
