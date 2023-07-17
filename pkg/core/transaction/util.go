package transaction

import (
	"github.com/ethereum/go-ethereum/rlp"
)

const EthLegacyBaseLength = 100

type writeCounter int

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

func RlpSize(v interface{}) int {
	c := writeCounter(0)
	rlp.Encode(&c, v)
	return int(c)
}

func CalculateNetworkFee(tx *Transaction, feePerByte uint64) uint64 {
	size := EthLegacyBaseLength + len(tx.Data())
	return uint64(size) * feePerByte
}
