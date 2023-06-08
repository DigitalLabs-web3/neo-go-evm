package statedb

import (
	"fmt"
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/util/slice"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type MemStore struct {
	*dao.Simple
	fund       *big.Int
	preimages  map[common.Hash][]byte
	logs       []*types.Log
	accesslist *accessList
}

func (m *MemStore) clone() *MemStore {
	mc := &MemStore{
		Simple:     m.Simple.GetPrivate(),
		fund:       big.NewInt(m.fund.Int64()),
		preimages:  make(map[common.Hash][]byte, len(m.preimages)),
		logs:       make([]*types.Log, len(m.logs)),
		accesslist: m.accesslist.Copy(),
	}
	for k, v := range m.preimages {
		mc.preimages[k] = slice.Copy(v)
	}
	copy(mc.logs, m.logs)
	return mc
}

type StateDB struct {
	Err       error
	bc        NativeContracts
	snapshot  int
	memStores []*MemStore
	ps        MemStore
}

func NewStateDB(ps *dao.Simple, bc NativeContracts) *StateDB {
	return &StateDB{
		Err:      nil,
		bc:       bc,
		snapshot: -1,
		ps: MemStore{
			Simple:     ps,
			fund:       big.NewInt(0),
			preimages:  make(map[common.Hash][]byte),
			accesslist: newAccessList(),
			logs:       []*types.Log{},
		},
		memStores: []*MemStore{},
	}
}

func (s *StateDB) CurrentStore() *MemStore {
	if s.snapshot < 0 {
		return &s.ps
	}
	return s.memStores[s.snapshot]
}

func (s *StateDB) Snapshot() int {
	s.memStores = append(s.memStores, s.CurrentStore().clone())
	s.snapshot++
	return s.snapshot
}

func (s *StateDB) RevertToSnapshot(revid int) {
	if revid > s.snapshot || revid < 0 {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	s.memStores = s.memStores[:revid]
	s.snapshot = revid - 1
}

func (s *StateDB) GetCommittedState(address common.Address, key common.Hash) common.Hash {
	item := s.ps.GetStorageItem(address, key.Bytes())
	return common.BytesToHash(item)
}

func (s *StateDB) CreateAccount(common.Address) {}

func (s *StateDB) SubBalance(address common.Address, amount *big.Int) {
	s.bc.Contracts().GAS.SubBalance(s.CurrentStore().Simple, address, amount)
}

func (s *StateDB) AddBalance(address common.Address, amount *big.Int) {
	s.bc.Contracts().GAS.AddBalance(s.CurrentStore().Simple, address, amount)
}

func (s *StateDB) GetBalance(address common.Address) *big.Int {
	return s.bc.Contracts().GAS.GetBalance(s.CurrentStore().Simple, address)
}

func (s *StateDB) GetNonce(address common.Address) uint64 {
	return s.bc.Contracts().Ledger.GetNonce(s.CurrentStore().Simple, address)
}

func (s *StateDB) SetNonce(address common.Address, nonce uint64) {
	s.bc.Contracts().Ledger.SetNonce(s.CurrentStore().Simple, address, nonce)
}

func (s *StateDB) GetCodeHash(address common.Address) common.Hash {
	return common.Hash(s.bc.Contracts().Management.GetCodeHash(s.CurrentStore().Simple, address))
}

func (s *StateDB) GetCode(address common.Address) []byte {
	return s.bc.Contracts().Management.GetCode(s.CurrentStore().Simple, address)
}

func (s *StateDB) SetCode(address common.Address, code []byte) {
	s.bc.Contracts().Management.SetCode(s.CurrentStore().Simple, address, code)
}

func (s *StateDB) GetCodeSize(address common.Address) int {
	return s.bc.Contracts().Management.GetCodeSize(s.CurrentStore().Simple, address)
}

func (s *StateDB) AddRefund(amount uint64) {
	s.CurrentStore().fund.Add(s.CurrentStore().fund, new(big.Int).SetUint64(amount))
}

func (s *StateDB) SubRefund(amount uint64) {
	s.CurrentStore().fund.Sub(s.CurrentStore().fund, new(big.Int).SetUint64(amount))
}

func (s *StateDB) GetRefund() uint64 {
	return s.CurrentStore().fund.Uint64()
}

func (s *StateDB) GetState(address common.Address, key common.Hash) common.Hash {
	item := s.CurrentStore().GetStorageItem(address, key.Bytes())
	if item == nil {
		return common.Hash{}
	}
	return common.BytesToHash(item)
}

func (s *StateDB) SetState(address common.Address, key common.Hash, value common.Hash) {
	s.CurrentStore().PutStorageItem(address, key.Bytes(), value.Bytes())
}

func (s *StateDB) Suicide(address common.Address) bool {
	return s.bc.Contracts().Management.Destroy(s.CurrentStore().Simple, address)
}

func (s *StateDB) HasSuicided(address common.Address) bool {
	sc := s.bc.Contracts().Management.GetContract(s.CurrentStore().Simple, address)
	return sc == nil
}

func (s *StateDB) Exist(address common.Address) bool {
	return !s.Empty(address)
}

func (s *StateDB) Empty(address common.Address) bool {
	if s.bc.Contracts().GAS.GetBalance(s.CurrentStore().Simple, address).Sign() > 0 {
		return false
	}
	if sc := s.bc.Contracts().Management.GetContract(s.CurrentStore().Simple, address); sc != nil {
		return false
	}
	if n := s.bc.Contracts().Ledger.GetNonce(s.CurrentStore().Simple, address); n != 0 {
		return false
	}
	return true
}

func (s *StateDB) PrepareAccessList(sender common.Address, dest *common.Address, precompiles []common.Address, list types.AccessList) {
	s.CurrentStore().accesslist = newAccessList()
	s.AddAddressToAccessList(sender)
	if dest != nil {
		s.AddAddressToAccessList(*dest)
	}
	for _, addr := range precompiles {
		s.AddAddressToAccessList(addr)
	}
	for _, el := range list {
		s.AddAddressToAccessList(el.Address)
		for _, key := range el.StorageKeys {
			s.AddSlotToAccessList(el.Address, key)
		}
	}
}

func (s *StateDB) AddressInAccessList(addr common.Address) bool {
	return s.CurrentStore().accesslist.ContainsAddress(addr)
}

func (s *StateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return s.CurrentStore().accesslist.Contains(addr, slot)
}

func (s *StateDB) AddAddressToAccessList(addr common.Address) {
	s.CurrentStore().accesslist.AddAddress(addr)
}

func (s *StateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	s.CurrentStore().accesslist.AddSlot(addr, slot)
}

func (s *StateDB) AddLog(l *types.Log) {
	s.CurrentStore().logs = append(s.CurrentStore().logs, l)
}

func (s *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	s.CurrentStore().preimages[hash] = preimage
}

func (s *StateDB) ForEachStorage(addr common.Address, cb func(common.Hash, common.Hash) bool) error {
	rng := storage.SeekRange{
		Prefix:    []byte{},
		Start:     []byte{},
		Backwards: false,
	}
	s.CurrentStore().Seek(addr, rng, func(k []byte, v []byte) bool {
		return cb(common.BytesToHash(k), common.BytesToHash(v))
	})
	return nil
}

func (s *StateDB) GetLogs() []*types.Log {
	return s.CurrentStore().logs
}

func (s *StateDB) Commit() error {
	if s.Err != nil {
		return s.Err
	}
	for s.snapshot >= 0 {
		_, err := s.CurrentStore().Persist()
		if err != nil {
			return err
		}
		s.memStores = s.memStores[:s.snapshot]
		s.snapshot--
	}
	return nil
}
