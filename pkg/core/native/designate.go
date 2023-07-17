package native

import (
	"encoding/binary"
	"math"
	"sort"
	"sync"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativeids"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativenames"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/noderoles"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/ethereum/go-ethereum/common"
)

const (
	MaxNodeCount = 21
)

var (
	DesignationAddress common.Address = common.Address(common.BytesToAddress([]byte{nativeids.Designation}))
)

type Designate struct {
	state.NativeContract
	StandbyValidators keys.PublicKeys
	cache             *DesignationCache
}

type roleData struct {
	nodes  keys.PublicKeys
	addr   common.Address
	height uint32
}

type DesignationCache struct {
	mutex            sync.Mutex
	rolesChangedFlag bool
	validators       roleData
	stateVals        roleData
}

func NewDesignate(cfg config.ProtocolConfiguration) *Designate {
	d := &Designate{
		NativeContract: state.NativeContract{
			Name: nativenames.Designation,
			Contract: state.Contract{
				Address:  DesignationAddress,
				CodeHash: hash.Keccak256(DesignationAddress[:]),
				Code:     DesignationAddress[:],
			},
		},
		StandbyValidators: cfg.StandbyValidators,
		cache:             &DesignationCache{},
	}
	designateAbi, contractCalls, err := constructAbi(d)
	if err != nil {
		panic(err)
	}
	d.Abi = *designateAbi
	d.ContractCalls = contractCalls
	return d
}

func (d *Designate) UpdateCache(s *dao.Simple) error {
	err := d.updateCachedRoleData(d.cache, s, noderoles.Validator)
	if err != nil {
		return err
	}
	return d.updateCachedRoleData(d.cache, s, noderoles.StateValidator)
}

func (d *Designate) PostPersist(s *dao.Simple, _ *block.Block) error {
	return d.UpdateCache(s)
}

func (d *Designate) updateCachedRoleData(cache *DesignationCache, s *dao.Simple, r noderoles.Role) error {
	d.cache.mutex.Lock()
	defer d.cache.mutex.Unlock()
	var v *roleData
	switch r {
	case noderoles.StateValidator:
		v = &cache.stateVals
	case noderoles.Validator:
		v = &cache.validators
	}
	nodeKeys, height, err := d.GetDesignatedByRoleFromStorage(s, r, math.MaxUint32)
	if err != nil {
		return err
	}
	v.nodes = nodeKeys
	addr, err := addressFromNodes(r, nodeKeys)
	if err != nil {
		return err
	}
	v.addr = addr
	v.height = height
	cache.rolesChangedFlag = true
	return nil
}

func addressFromNodes(r noderoles.Role, nodes keys.PublicKeys) (common.Address, error) {
	if nodes.Len() == 0 {
		return common.Address{}, nil
	}
	script, err := nodes.CreateDefaultMultiSigRedeemScript()
	if err != nil {
		return common.Address{}, err
	}
	return hash.Hash160(script), nil
}

func (d *Designate) Initialize(s *dao.Simple) error {
	s.PutStorageItem(d.Address, createRoleKey(noderoles.Validator, 0), d.StandbyValidators.Bytes())
	s.PutStorageItem(d.Address, createRoleKey(noderoles.StateValidator, 0), d.StandbyValidators.Bytes())
	return nil
}

// func (d *Designate) ContractCall_designateAsRole(ic InteropContext, role byte, rawPks []byte) error {
// 	r := noderoles.Role(role)
// 	pks := new(keys.PublicKeys)
// 	err := pks.DecodeBytes(rawPks)
// 	if err != nil {
// 		return err
// 	}
// 	err = d.checkConsensus(ic)
// 	if err != nil {
// 		return err
// 	}
// 	err = d.designateAsRole(ic, r, *pks)
// 	if err == nil {
// 		log(ic, d.Address, rawPks, d.Abi.Events["designateAsRole"].ID, common.BytesToHash([]byte{role}))
// 	}
// 	return err
// }

func (d *Designate) checkConsensus(ic InteropContext) error {
	if ic.PersistingBlock() == nil {
		return ErrNoBlock
	}
	consensus, err := d.GetConsensusAddress(ic.Dao(), ic.PersistingBlock().Index)
	if err != nil {
		return err
	}
	if ic.Sender() != consensus {
		return ErrInvalidSender
	}
	return nil
}

func (d *Designate) designateAsRole(ic InteropContext, r noderoles.Role, keys keys.PublicKeys) error {
	if !noderoles.IsValid(r) {
		return ErrInvalidRole
	}
	ks := keys.Unique()
	if ks.Len() == 0 {
		return ErrEmptyNodeList
	}
	if ks.Len() > MaxNodeCount {
		return ErrLargeNodeList
	}
	index := ic.PersistingBlock().Index + 2
	sort.Sort(ks)
	ic.Dao().PutStorageItem(d.Address, createRoleKey(r, index), ks.Bytes())
	return nil
}

func createRoleKey(role noderoles.Role, index uint32) []byte {
	key := make([]byte, 5)
	key[0] = byte(role)
	binary.LittleEndian.PutUint32(key[1:], index)
	return key
}

func (d *Designate) GetDesignatedByRole(s *dao.Simple, r noderoles.Role, index uint32) (keys.PublicKeys, uint32, error) {
	if !noderoles.IsValid(r) {
		return nil, 0, ErrInvalidRole
	}
	d.cache.mutex.Lock()
	defer d.cache.mutex.Unlock()
	var val roleData
	switch r {
	case noderoles.Validator:
		val = d.cache.validators
	case noderoles.StateValidator:
		val = d.cache.stateVals
	default:
		return nil, 0, ErrInvalidRole
	}
	if val.height <= index {
		return val.nodes.Copy(), val.height, nil
	}
	return d.GetDesignatedByRoleFromStorage(s, r, index)
}

func (d *Designate) GetDesignatedByRoleFromStorage(s *dao.Simple, r noderoles.Role, index uint32) (keys.PublicKeys, uint32, error) {
	kvs, err := s.GetStorageItemsWithPrefix(d.Address, []byte{byte(r)})
	if err != nil {
		return nil, 0, err
	}
	var (
		ks     = keys.PublicKeys{}
		resSi  state.StorageItem
		height uint32
	)
	for i := len(kvs) - 1; i >= 0; i-- {
		kv := kvs[i]
		if len(kv.Key) < 4 {
			continue
		}
		siInd := binary.LittleEndian.Uint32(kv.Key)
		if siInd <= index {
			height = siInd
			resSi = kv.Item
			break
		}
	}
	if resSi != nil {
		err := ks.DecodeBytes(resSi)
		if err != nil {
			return nil, 0, err
		}
	}
	return ks, height, nil
}

func (d *Designate) GetValidators(s *dao.Simple, index uint32) (keys.PublicKeys, error) {
	validators, _, err := d.GetDesignatedByRole(s, noderoles.Validator, index)
	return validators, err
}

func (d *Designate) GetConsensusAddress(s *dao.Simple, index uint32) (common.Address, error) {
	validators, err := d.GetValidators(s, index)
	if err != nil {
		return common.Address{}, err
	}
	script, err := validators.CreateDefaultMultiSigRedeemScript()
	if err != nil {
		return common.Address{}, err
	}
	return hash.Hash160(script), nil
}

func (d *Designate) GetSysAddresses(s *dao.Simple, index uint32) ([]common.Address, error) {
	accounts, err := d.GetValidators(s, index)
	if err != nil {
		return nil, err
	}
	addrs := make([]common.Address, len(accounts))
	for i, account := range accounts {
		addrs[i] = account.Address()
	}
	script, err := keys.PublicKeys(accounts[:len(d.StandbyValidators)]).CreateDefaultMultiSigRedeemScript()
	if err != nil {
		return nil, err
	}
	addrs = append(addrs, hash.Hash160(script))
	return addrs, nil
}

func (d *Designate) RequiredGas(ic InteropContext, input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := d.Abi.MethodById(input[:4])
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

func (d *Designate) Run(ic InteropContext, input []byte) ([]byte, error) {
	return contractCall(d, &d.NativeContract, ic, input)
}
