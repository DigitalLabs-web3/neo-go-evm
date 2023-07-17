package native

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativeids"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativenames"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

const (
	prefixContract = 0x01
)

var ManagementAddress common.Address = common.Address(common.BytesToAddress([]byte{nativeids.Management}))

type Management struct {
	state.NativeContract
	cs *Contracts
}

func NewManagement(cs *Contracts) *Management {
	m := &Management{
		NativeContract: state.NativeContract{
			Name: nativenames.Management,
			Contract: state.Contract{
				Address:  ManagementAddress,
				CodeHash: hash.Keccak256(ManagementAddress[:]),
				Code:     ManagementAddress[:],
			},
		},
		cs: cs,
	}
	mAbi, contractCalls, err := constructAbi(m)
	if err != nil {
		panic(err)
	}
	m.Abi = *mAbi
	m.ContractCalls = contractCalls
	return m
}

func (m *Management) Initialize(d *dao.Simple) error {
	for _, native := range m.cs.Contracts {
		item, err := io.ToByteArray(&native.Contract)
		if err != nil {
			return err
		}
		d.PutStorageItem(m.Address, MakeContractKey(native.Address), item)
	}
	return nil
}

func (m *Management) GetContract(s *dao.Simple, addr common.Address) *state.Contract {
	item := s.GetStorageItem(m.Address, MakeContractKey(addr))
	if item == nil {
		return nil
	}
	contract := &state.Contract{}
	err := io.FromByteArray(contract, item)
	if err != nil {
		panic(err)
	}
	return contract
}

func (m *Management) GetCode(s *dao.Simple, addr common.Address) []byte {
	contract := m.GetContract(s, addr)
	if contract == nil {
		return nil
	}
	return contract.Code
}

func (m *Management) GetCodeHash(s *dao.Simple, addr common.Address) common.Hash {
	contract := m.GetContract(s, addr)
	if contract == nil {
		return common.Hash{}
	}
	return contract.CodeHash
}

func (m *Management) GetCodeSize(s *dao.Simple, addr common.Address) int {
	contract := m.GetContract(s, addr)
	if contract == nil {
		return 0
	}
	return len(contract.Code)
}

func (m *Management) SetCode(s *dao.Simple, addr common.Address, code []byte) {
	contract := m.GetContract(s, addr)
	if contract == nil {
		contract = &state.Contract{
			Address: addr,
		}
	}
	contract.Code = code
	contract.CodeHash = hash.Keccak256(code)
	bytes, err := io.ToByteArray(contract)
	if err != nil {
		panic(err)
	}
	s.PutStorageItem(m.Address, MakeContractKey(addr), bytes)
}

func (m *Management) Destroy(s *dao.Simple, addr common.Address) bool {
	contract := m.GetContract(s, addr)
	if contract == nil {
		return false
	}
	k := MakeContractKey(addr)
	s.DeleteStorageItem(addr, k)
	return true
}

func (m *Management) RequiredGas(ic InteropContext, input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := m.Abi.MethodById(input[:4])
	if err != nil {
		return 0
	}
	switch method.Name {
	case "initialize":
		return 0
	default:
		return 0
	}
}

func (m *Management) Run(ic InteropContext, input []byte) ([]byte, error) {
	return contractCall(m, &m.NativeContract, ic, input)
}
