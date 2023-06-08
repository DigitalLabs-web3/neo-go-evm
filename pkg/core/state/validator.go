package state

import (
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
)

// Validator holds the state of a validator (its key and votes balance).
type Validator struct {
	Key   *keys.PublicKey
	Votes *big.Int
}
