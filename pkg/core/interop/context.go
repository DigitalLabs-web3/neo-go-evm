package interop

import (
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/statedb"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type NativeContract interface {
	RequiredGas(ic native.InteropContext, input []byte) uint64
	Run(ic native.InteropContext, input []byte) ([]byte, error)
}

type Chain interface {
	GetConfig() config.ProtocolConfiguration
	Contracts() *native.Contracts
	GetCurrentValidators() ([]*keys.PublicKey, error)
}

// Context represents context in which interops are executed.
type Context struct {
	Chain  Chain
	Block  *block.Block
	Tx     *transaction.Transaction
	VM     *EVM
	bctx   vm.BlockContext
	sdb    *statedb.StateDB
	caller common.Address
}

func NewContext(block *block.Block, tx *transaction.Transaction, sdb *statedb.StateDB, chain Chain) (*Context, error) {
	ctx := &Context{
		Chain:  chain,
		Block:  block,
		Tx:     tx,
		sdb:    sdb,
		caller: tx.From(),
	}
	ctx.bctx = newEVMBlockContext(block, chain, chain.GetConfig())
	txContext := vm.TxContext{
		Origin:   tx.From(),
		GasPrice: tx.GasPrice(),
	}
	ctx.VM = NewEVM(ctx.bctx,
		txContext, sdb, chain.GetConfig(),
		map[common.Address]vm.NativeContract{
			native.DesignationAddress: nativeWrapper{
				nativeContract: chain.Contracts().Designate,
				ic:             ctx,
			},
			native.PolicyAddress: nativeWrapper{
				nativeContract: chain.Contracts().Policy,
				ic:             ctx,
			},
			native.GASAddress: nativeWrapper{
				nativeContract: chain.Contracts().GAS,
				ic:             ctx,
			},
			native.ManagementAddress: nativeWrapper{
				nativeContract: chain.Contracts().Management,
				ic:             ctx,
			},
			native.BridgeAddress: nativeWrapper{
				nativeContract: chain.Contracts().Bridge,
				ic:             ctx,
			},
		})
	return ctx, nil
}

func newEVMBlockContext(block *block.Block,
	bc Chain,
	protocolSettings config.ProtocolConfiguration) (bctx vm.BlockContext) {
	var coinbase common.Address
	if block.Index > 0 {
		validators, err := bc.GetCurrentValidators()
		if err != nil {
			panic(err)
		}
		if len(validators) == 0 {
			panic("no validators")
		}
		coinbase = validators[block.PrimaryIndex].Address()
	}
	random := common.BigToHash(big.NewInt(int64(block.Nonce)))
	bctx = vm.BlockContext{
		CanTransfer: func(sdb vm.StateDB, from common.Address, amount *big.Int) bool {
			return sdb.GetBalance(from).Cmp(amount) > 0
		},
		Transfer: func(sdb vm.StateDB, from common.Address, to common.Address, amount *big.Int) {
			fromAmount := big.NewInt(0).Neg(amount)
			sdb.AddBalance(from, fromAmount)
			sdb.AddBalance(to, amount)
		},
		Coinbase:    coinbase,
		GasLimit:    uint64(protocolSettings.MaxBlockGas),
		BlockNumber: big.NewInt(int64(block.Index)),
		Time:        big.NewInt(int64(block.Timestamp)),
		Difficulty:  big.NewInt(0),
		BaseFee:     big.NewInt(0),
		Random:      &random,
	}
	return
}

func (c Context) Log(log *types.Log) {
	c.sdb.AddLog(log)
}

func (c Context) Sender() common.Address {
	return c.caller
}

func (c Context) Dao() *dao.Simple {
	return c.sdb.CurrentStore().Simple
}

func (c Context) PersistingBlock() *block.Block {
	return c.Block
}

func (c Context) Coinbase() common.Address {
	return c.bctx.Coinbase
}

func (c Context) Address() common.Address {
	return c.Tx.From()
}

func (c Context) Container() *transaction.Transaction {
	return c.Tx
}
