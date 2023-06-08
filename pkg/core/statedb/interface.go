package statedb

import "github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native"

type NativeContracts interface {
	Contracts() *native.Contracts
}
