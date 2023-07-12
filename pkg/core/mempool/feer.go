package mempool

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Feer is an interface that abstract the implementation of the fee calculation.
type Feer interface {
	FeePerByte() uint64
	GetGasPrice() *big.Int
	GetUtilityTokenBalance(common.Address) *big.Int
	BlockHeight() uint32
}
