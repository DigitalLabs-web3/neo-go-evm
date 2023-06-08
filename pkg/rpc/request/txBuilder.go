package request

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// ExpandArrayIntoScript pushes all FuncParam parameters from the given array
// into the given buffer in reverse order.
func ExpandArrayIntoScript(script *io.BinWriter, slice []Param) error {

	return script.Err
}

// CreateFunctionInvocationScript creates a script to invoke given contract with
// given parameters.
func CreateFunctionInvocationScript(contract common.Address, method string, param *Param) ([]byte, error) {
	script := io.NewBufBinWriter()
	return script.Bytes(), nil
}
