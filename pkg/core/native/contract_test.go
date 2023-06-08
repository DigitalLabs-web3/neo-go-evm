package native

import (
	"reflect"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	d := dao.NewSimple(storage.NewMemoryStore())
	ledger := NewLedger()
	management := NewManagement(nil)
	t.Logf("%s, %s\n", LedgerAddress, ManagementAddress)
	ledger.SetNonce(d, common.Address{}, 1)
	code := management.GetCode(d, common.Address{})
	assert.Nil(t, code)
}

func TestAddressType(t *testing.T) {
	addr := common.Hash{}
	ty := reflect.TypeOf(addr)
	t.Log(ty.Name())
	t.Log(ty.Kind())
	t.Log(ty.Size())
	t.Log(ty.Elem().Kind())
	t.Log(ty)
}
