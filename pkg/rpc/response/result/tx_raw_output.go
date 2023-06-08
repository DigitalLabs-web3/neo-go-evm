package result

import (
	"encoding/json"
	"errors"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// TransactionOutputRaw is used as a wrapper to represents
// a Transaction.
type TransactionOutputRaw struct {
	transaction.Transaction
	TransactionMetadata
}

// TransactionMetadata is an auxiliary struct for proper TransactionOutputRaw marshaling.
type TransactionMetadata struct {
	Blockhash        common.Hash  `json:"blockHash"`
	BlockNumber      hexutil.Uint `json:"blockNumber"`
	TransactionIndex hexutil.Uint `json:"transactionIndex"`
}

// NewTransactionOutputRaw returns a new ransactionOutputRaw object.
func NewTransactionOutputRaw(tx *transaction.Transaction, header *block.Header, receipt *types.Receipt) TransactionOutputRaw {
	result := TransactionOutputRaw{
		Transaction: *tx,
	}
	if header == nil {
		return result
	}
	result.TransactionMetadata = TransactionMetadata{
		Blockhash:        header.Hash(),
		BlockNumber:      hexutil.Uint(header.Index),
		TransactionIndex: hexutil.Uint(receipt.TransactionIndex),
	}
	return result
}

// MarshalJSON implements json.Marshaler interface.
func (t TransactionOutputRaw) MarshalJSON() ([]byte, error) {
	output, err := json.Marshal(t.TransactionMetadata)
	if err != nil {
		return nil, err
	}
	txBytes, err := json.Marshal(&t.Transaction)
	if err != nil {
		return nil, err
	}

	// We have to keep both transaction.Transaction and tranactionOutputRaw at the same level in json
	// in order to match C# API, so there's no way to marshall Tx correctly with standard json.Marshaller tool.
	if output[len(output)-1] != '}' || txBytes[0] != '{' {
		return nil, errors.New("can't merge internal jsons")
	}
	output[len(output)-1] = ','
	output = append(output, txBytes[1:]...)
	return output, nil
}

// UnmarshalJSON implements json.Marshaler interface.
func (t *TransactionOutputRaw) UnmarshalJSON(data []byte) error {
	// As transaction.Transaction and tranactionOutputRaw are at the same level in json,
	// do unmarshalling separately for both structs.
	output := new(TransactionMetadata)
	err := json.Unmarshal(data, output)
	if err != nil {
		return err
	}
	t.TransactionMetadata = *output
	return json.Unmarshal(data, &t.Transaction)
}
