package interop

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type nativeWrapper struct {
	nativeContract NativeContract
	ic             *Context
}

func (w nativeWrapper) RequiredGas(input []byte) uint64 {
	return w.nativeContract.RequiredGas(w.ic, input)
}

func (w nativeWrapper) Run(caller common.Address, input []byte, value *big.Int) ([]byte, error) {
	w.ic.caller = caller
	if value == nil {
		w.ic.value = *big.NewInt(0)
	} else {
		w.ic.value = *value
	}
	return w.nativeContract.Run(w.ic, input)
}
