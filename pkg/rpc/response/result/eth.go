package result

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Syncing struct {
	StartingBlock string `json:"startingBlock"`
	CurrentBlock  string `json:"currentBlock"`
	HighestBlock  string `json:"highestBlock"`
}

type TransactionObject struct {
	Data     []byte               `json:"data"`     //optional
	From     common.Address       `json:"from"`     //optional
	Gas      uint64               `json:"gas"`      //optional
	GasPrice *big.Int             `json:"gasPrice"` //optional
	To       *common.Address      `json:"to"`
	Value    *big.Int             `json:"value"`   //optional
	Witness  *transaction.Witness `json:"witness"` //optional
}

type txObj struct {
	Data     string               `json:"data"`
	From     string               `json:"from"`
	Gas      string               `json:"gas"`
	GasPrice string               `json:"gasPrice"`
	To       string               `json:"to,omitempty"`
	Value    string               `json:"value"`
	Witness  *transaction.Witness `json:"witness"`
}

func (t *TransactionObject) UnmarshalJSON(text []byte) error {
	tx := txObj{}
	err := json.Unmarshal(text, &tx)
	if err != nil {
		return err
	}
	t.From = common.HexToAddress(tx.From)
	if len(tx.To) == 0 {
		t.To = nil
	} else {
		to := common.HexToAddress(tx.To)
		t.To = &to
	}
	if len(tx.Gas) == 0 {
		t.Gas = 0
	} else {
		gas, err := hexutil.DecodeUint64(tx.Gas)
		if err != nil {
			return err
		}
		t.Gas = gas
	}
	if len(tx.GasPrice) == 0 {
		t.GasPrice = big.NewInt(0)
	} else {
		gasPrice, err := hexutil.DecodeBig(tx.GasPrice)
		if err != nil {
			return err
		}
		t.GasPrice = gasPrice
	}
	if len(tx.Value) == 0 {
		t.Value = big.NewInt(0)
	} else {
		val, err := hexutil.DecodeBig(tx.Value)
		if err != nil {
			return err
		}
		t.Value = val
	}
	if len(tx.Data) == 0 {
		t.Data = nil
	} else {
		data, err := hexutil.Decode(tx.Data)
		if err != nil {
			return err
		}
		t.Data = data
	}
	if tx.Witness == nil {
		t.Witness = nil
	} else {
		t.Witness = tx.Witness
	}
	return nil
}

func (t TransactionObject) MarshalJSON() ([]byte, error) {
	if t.GasPrice == nil {
		return nil, errors.New("gasPrice can't be nil")
	}
	if t.Value == nil {
		return nil, errors.New("value can't be nil")
	}
	tx := txObj{
		From:     t.From.String(),
		Gas:      hexutil.EncodeUint64(t.Gas),
		GasPrice: hexutil.EncodeBig(t.GasPrice),
		Data:     hexutil.Encode(t.Data),
		Value:    hexutil.EncodeBig(t.Value),
		Witness:  t.Witness,
	}
	if t.To != nil {
		tx.To = t.To.String()
	}
	return json.Marshal(tx)
}
