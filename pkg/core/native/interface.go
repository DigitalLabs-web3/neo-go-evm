package native

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type InteropContext interface {
	Log(*types.Log)
	Sender() common.Address
	Dao() *dao.Simple
	Container() *transaction.Transaction
	PersistingBlock() *block.Block
}
