package result

import (
	"encoding/json"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type receptExtra struct {
	From common.Address  `json:"from"`
	To   *common.Address `json:"to"`
}

type RReceipt struct {
	receptExtra
	types.Receipt
}

func NewRReceipt(receipt *types.Receipt, tx *transaction.Transaction) RReceipt {
	return RReceipt{
		receptExtra: receptExtra{
			From: tx.From(),
			To:   tx.To(),
		},
		Receipt: *receipt,
	}
}

func (rr RReceipt) MarshalJSON() ([]byte, error) {
	output, err := json.Marshal(rr.Receipt)
	if err != nil {
		return nil, err
	}
	extrabytes, err := json.Marshal(rr.receptExtra)
	if err != nil {
		return nil, err
	}
	output[len(output)-1] = ','
	output = append(output, extrabytes[1:]...)
	return output, nil
}
