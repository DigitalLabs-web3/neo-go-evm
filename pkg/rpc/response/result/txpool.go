package result

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type poolTx struct {
	BlockHash        common.Hash     `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              hexutil.Uint64  `json:"gas"`
	GasPrice         hexutil.Big     `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint64  `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex *hexutil.Big    `json:"transactionIndex"`
	Value            hexutil.Big     `json:"value"`
	R                hexutil.Big     `json:"r"`
	S                hexutil.Big     `json:"s"`
	V                hexutil.Big     `json:"v"`
}

type TxPool struct {
	Pending map[string]map[uint64]poolTx `json:"pending"`
	Queued  map[string]map[uint64]poolTx `json:"queued"`
}

func NewTxPool(txes []*transaction.Transaction) TxPool {
	tp := TxPool{
		Pending: make(map[string]map[uint64]poolTx),
		Queued:  make(map[string]map[uint64]poolTx),
	}
	for _, tx := range txes {
		from := tx.From().String()
		pooltx := poolTx{
			BlockHash:        common.Hash{},
			BlockNumber:      nil,
			From:             tx.From(),
			Gas:              hexutil.Uint64(tx.Gas()),
			GasPrice:         hexutil.Big(*tx.GasPrice()),
			Hash:             tx.Hash(),
			Input:            tx.Data(),
			Nonce:            hexutil.Uint64(tx.Nonce()),
			To:               tx.To(),
			TransactionIndex: nil,
			Value:            hexutil.Big(*tx.Value()),
		}
		if tx.Type == transaction.EthTxType {
			r, s, v := tx.EthTx.RawSignatureValues()
			pooltx.R = hexutil.Big(*r)
			pooltx.S = hexutil.Big(*s)
			pooltx.V = hexutil.Big(*v)
		}
		noncetable, ok := tp.Pending[from]
		if !ok {
			noncetable = make(map[uint64]poolTx)
			tp.Pending[from] = noncetable
		}
		noncetable[tx.Nonce()] = pooltx
	}
	return tp
}
