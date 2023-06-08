package transaction

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

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
	switch tx.Type {
	case EthTxType:
		t := tx.EthTx
		size := EthLegacyBaseLength + len(t.Data())
		return uint64(size) * feePerByte
	case NeoTxType:
		t := tx.NeoTx
		size := 8 +
			io.GetVarSize(t.GasPrice.Bytes()) +
			8 +
			common.AddressLength +
			io.GetVarSize(t.Value.Bytes()) +
			io.GetVarSize(t.Data) +
			1 //from
		if t.To != nil {
			size += common.AddressLength
		}
		size += io.GetVarSize(t.Witness.VerificationScript)
		if t.Witness.VerificationScript[0] == 0 {
			size += SignatureLength + 1
		} else {
			size += 1 + int(t.Witness.VerificationScript[0])*(SignatureLength+1)
		}
		return uint64(size) * feePerByte
	default:
		return 0
	}
}
