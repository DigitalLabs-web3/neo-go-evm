package native

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/noderoles"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

func TestEndian(t *testing.T) {
	var index uint32 = 1
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, index)
	t.Log(hex.EncodeToString(b))
}

type interopContext struct {
	D         *dao.Simple
	S         common.Address
	Index     uint32
	Contracts *Contracts
	L         []*types.Log
}

func (ic interopContext) Log(l *types.Log) {
	ic.L[0] = l
}

func (ic interopContext) Sender() common.Address {
	return ic.S
}

func (ic interopContext) Value() big.Int {
	return *big.NewInt(0)
}

func (ic interopContext) Natives() *Contracts {
	if ic.Contracts != nil {
		return ic.Contracts
	}
	return &Contracts{}
}

func (ic interopContext) Dao() *dao.Simple {
	return ic.D
}

func (ic interopContext) Container() *transaction.Transaction {
	return nil
}

func (ic interopContext) PersistingBlock() *block.Block {
	return &block.Block{
		Header: block.Header{
			Index: ic.Index,
		},
	}
}

func TestCommitteeRole(t *testing.T) {
	pubs, _ := keys.NewPublicKeysFromStrings([]string{
		"023c4d39a3fd2150407a9d4654430cdce0464eccaaf739eea79d63e2862f989ee6",
	})
	dao := dao.NewSimple(storage.NewMemoryStore())
	des := NewDesignate(config.ProtocolConfiguration{
		StandbyValidators: pubs,
	})
	k1, err := keys.NewPublicKeyFromString("023c4d39a3fd2150407a9d4654430cdce0464eccaaf739eea79d63e2862f989ee6")
	assert.NoError(t, err)
	ic := interopContext{
		D: dao,
	}
	err = des.Initialize(dao)
	assert.NoError(t, err)
	ks, _, err := des.GetDesignatedByRole(dao, noderoles.Validator, 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, ks.Len())
	assert.Equal(t, "023c4d39a3fd2150407a9d4654430cdce0464eccaaf739eea79d63e2862f989ee6", hex.EncodeToString(ks[0].Bytes()))
	// - change one committee -
	k2, err := keys.NewPublicKeyFromString("0218cbadb9db833a6b7432a920b6bdb6b822eb2df0d59cfc5d9d590d5dfd97fef4")
	assert.NoError(t, err)
	s, err := des.GetConsensusAddress(dao, 1)
	assert.NoError(t, err)
	ic.S = s
	ic.Index = 1
	err = des.designateAsRole(ic, noderoles.Validator, keys.PublicKeys{k2})
	assert.NoError(t, err)
	ks, _, err = des.GetDesignatedByRole(dao, noderoles.Validator, 1)
	assert.NoError(t, err)
	assert.Equal(t, "023c4d39a3fd2150407a9d4654430cdce0464eccaaf739eea79d63e2862f989ee6", hex.EncodeToString(ks[0].Bytes()))
	ks, _, err = des.GetDesignatedByRole(dao, noderoles.Validator, 3)
	assert.NoError(t, err)
	assert.Equal(t, "0218cbadb9db833a6b7432a920b6bdb6b822eb2df0d59cfc5d9d590d5dfd97fef4", hex.EncodeToString(ks[0].Bytes()))
	// - - - - - - - - - - - - -
	// - change committee to 2 from 1 -
	s, err = des.GetConsensusAddress(dao, 101)
	assert.NoError(t, err)
	ic.S = s
	ic.Index = 102
	err = des.designateAsRole(ic, noderoles.Validator, keys.PublicKeys{k1, k2})
	assert.NoError(t, err)
	ks, _, err = des.GetDesignatedByRole(dao, noderoles.Validator, 101)
	assert.NoError(t, err)
	assert.Equal(t, 1, ks.Len())
	ks, _, err = des.GetDesignatedByRole(dao, noderoles.Validator, 104)
	assert.NoError(t, err)
	assert.Equal(t, 2, ks.Len())
}

func TestDesignateContractCall(t *testing.T) {
	pubs, _ := keys.NewPublicKeysFromStrings([]string{
		"023c4d39a3fd2150407a9d4654430cdce0464eccaaf739eea79d63e2862f989ee6",
	})
	dao := dao.NewSimple(storage.NewMemoryStore())
	des := NewDesignate(config.ProtocolConfiguration{
		StandbyValidators: pubs,
	})
	ic := interopContext{
		D: dao,
		L: make([]*types.Log, 1),
	}
	err := des.Initialize(dao)
	assert.NoError(t, err)
	ic.S, _ = des.GetConsensusAddress(dao, 1)
	fn, ok := des.Abi.Methods["designateAsRole"]
	assert.True(t, ok)
	input := append(fn.ID, []byte{0, 0}...)
	_, err = des.Run(ic, input)
	assert.NotNil(t, err)

	k, err := keys.NewPublicKeyFromString("0218cbadb9db833a6b7432a920b6bdb6b822eb2df0d59cfc5d9d590d5dfd97fef4")
	ks := keys.PublicKeys{k}
	assert.NoError(t, err)
	input, err = des.Abi.Pack("designateAsRole", uint8(0), ks.Bytes())
	assert.NoError(t, err)
	_, err = des.Run(ic, input)
	assert.NoError(t, err)

	assert.Equal(t, ic.L[0].Address, des.Address)
	assert.Equal(t, 2, len(ic.L[0].Topics))
	assert.Equal(t, des.Abi.Events["designateAsRole"].ID, ic.L[0].Topics[0])
	assert.Equal(t, common.BytesToHash([]byte{byte(noderoles.Validator)}), ic.L[0].Topics[1])
	assert.Equal(t, ks.Bytes(), ic.L[0].Data)

	ks, _, err = des.GetDesignatedByRole(dao, noderoles.Validator, 2)
	assert.NoError(t, err)
	assert.Equal(t, "0218cbadb9db833a6b7432a920b6bdb6b822eb2df0d59cfc5d9d590d5dfd97fef4", hex.EncodeToString(ks[0].Bytes()))
}

func TestMarshalNativeAbi(t *testing.T) {
	pubs, _ := keys.NewPublicKeysFromStrings([]string{
		"023c4d39a3fd2150407a9d4654430cdce0464eccaaf739eea79d63e2862f989ee6",
	})
	des := NewDesignate(config.ProtocolConfiguration{
		StandbyValidators: pubs,
	})
	b, err := json.Marshal(des)
	assert.NoError(t, err)
	t.Log(string(b))
	d := Designate{}
	err = json.Unmarshal(b, &d)
	assert.NoError(t, err)
}
