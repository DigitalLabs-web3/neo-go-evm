package transaction

import (
	"encoding/json"
	"errors"
	"math/big"

	nio "github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	ErrInvalidChainID = errors.New("invalid chainId")
)

type EthTx struct {
	types.Transaction
	ChainID uint64
	Sender  common.Address
}

func NewEthTx(tx *types.Transaction) (*EthTx, error) {
	t := &EthTx{
		Transaction: *tx,
	}
	var err error
	t.ChainID, t.Sender, err = t.deriveSigned()
	if err != nil {
		return nil, err
	}
	return t, nil
}

func NewEthTxFromBytes(data []byte) (*EthTx, error) {
	tx := new(types.Transaction)
	err := tx.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}
	return NewEthTx(tx)
}

func (t *EthTx) WithSignature(chainId uint64, sig []byte) error {
	signer := types.NewEIP155Signer(big.NewInt(int64(chainId)))
	tx, err := t.Transaction.WithSignature(signer, sig)
	t.Transaction = *tx
	return err
}

func (t *EthTx) IsValid() error {
	if t.Value().Sign() < 0 {
		return ErrNegativeValue
	}
	if t.Value().BitLen() > 256 {
		return ErrValueVeryHigh
	}
	if t.GasFeeCap().BitLen() > 256 {
		return ErrFeeCapVeryHigh
	}
	if t.GasTipCap().BitLen() > 256 {
		return ErrTipVeryHigh
	}
	if t.GasTipCap().Cmp(t.GasFeeCap()) > 0 {
		return ErrTipAboveFeeCap
	}
	if t.GasPrice().BitLen() > 256 {
		return ErrGasPriceVeryHigh
	}
	return nil
}

func (t *EthTx) Verify(chainId uint64) (err error) {
	if t.ChainID == 0 && t.Sender == (common.Address{}) {
		t.ChainID, t.Sender, err = t.deriveSigned()
		if err != nil {
			return
		}
	}
	if t.ChainID != chainId {
		return ErrInvalidChainID
	}
	return nil
}

func (t *EthTx) EncodeBinary(w *nio.BinWriter) {
	w.Err = rlp.Encode(w, &t.Transaction)
}

func (t *EthTx) DecodeBinary(r *nio.BinReader) {
	var err error
	defer func() {
		r.Err = err
	}()
	err = rlp.Decode(r, &t.Transaction)
	if err != nil {
		return
	}
	t.ChainID, t.Sender, err = t.deriveSigned()
	if err != nil {
		return
	}
}

func (t EthTx) MarshalJSON() ([]byte, error) {
	v, r, s := t.Transaction.RawSignatureValues()
	tx := &EthTxJson{
		Type:    hexutil.Uint(t.Type()),
		Hash:    t.Hash(),
		Nonce:   hexutil.Uint64(t.Nonce()),
		Gas:     hexutil.Uint64(t.Gas()),
		To:      t.To(),
		Value:   hexutil.Big(*t.Value()),
		Data:    hexutil.Bytes(t.Data()),
		V:       hexutil.Big(*v),
		R:       hexutil.Big(*r),
		S:       hexutil.Big(*s),
		ChainID: hexutil.Uint(t.ChainID),
		Sender:  t.Sender,
	}
	if t.Transaction.Type() == types.DynamicFeeTxType {
		tx.GasFeeCap = (*hexutil.Big)(t.Transaction.GasFeeCap())
		tx.GasTipCap = (*hexutil.Big)(t.Transaction.GasTipCap())
	}
	tx.GasPrice = (*hexutil.Big)(t.GasPrice())
	if t.Transaction.Type() != types.LegacyTxType {
		al := t.Transaction.AccessList()
		tx.AccessList = &al
	}
	return json.Marshal(tx)
}

func (t *EthTx) UnmarshalJSON(data []byte) error {
	tx := new(EthTxJson)
	err := json.Unmarshal(data, tx)
	if err != nil {
		return err
	}
	t.ChainID = uint64(tx.ChainID)
	t.Sender = tx.Sender
	switch tx.Type {
	case types.LegacyTxType:
		ltx := &types.LegacyTx{
			Nonce:    uint64(tx.Nonce),
			GasPrice: (*big.Int)(tx.GasPrice),
			Gas:      uint64(tx.Gas),
			To:       tx.To,
			Value:    (*big.Int)(&tx.Value),
			Data:     tx.Data,
			V:        (*big.Int)(&tx.V),
			R:        (*big.Int)(&tx.R),
			S:        (*big.Int)(&tx.S),
		}
		t.Transaction = *types.NewTx(ltx)
	case types.AccessListTxType:
		atx := &types.AccessListTx{
			Nonce:      uint64(tx.Nonce),
			GasPrice:   (*big.Int)(tx.GasPrice),
			Gas:        uint64(tx.Gas),
			To:         tx.To,
			Value:      (*big.Int)(&tx.Value),
			AccessList: *tx.AccessList,
			Data:       tx.Data,
			V:          (*big.Int)(&tx.V),
			R:          (*big.Int)(&tx.R),
			S:          (*big.Int)(&tx.S),
		}
		t.Transaction = *types.NewTx(atx)
	case types.DynamicFeeTxType:
		dtx := &types.DynamicFeeTx{
			Nonce:      uint64(tx.Nonce),
			GasTipCap:  (*big.Int)(tx.GasTipCap),
			GasFeeCap:  (*big.Int)(tx.GasFeeCap),
			Gas:        uint64(tx.Gas),
			To:         tx.To,
			Value:      (*big.Int)(&tx.Value),
			AccessList: *tx.AccessList,
			Data:       tx.Data,
			V:          (*big.Int)(&tx.V),
			R:          (*big.Int)(&tx.R),
			S:          (*big.Int)(&tx.S),
		}
		t.Transaction = *types.NewTx(dtx)
	default:
		return ErrUnsupportType
	}
	// compare hash
	return nil
}

func (t *EthTx) deriveSigned() (chainId uint64, sender common.Address, err error) {
	bigChainId := t.Transaction.ChainId()
	if !bigChainId.IsUint64() {
		err = errors.New("ChainId is not uint64")
		return
	}
	chainId = bigChainId.Uint64()
	sender, err = t.deriveSender(chainId)
	if err != nil {
		return
	}
	return
}

func (t *EthTx) deriveSender(chainId uint64) (common.Address, error) {
	signer := types.NewLondonSigner(big.NewInt(int64(chainId)))
	return signer.Sender(&t.Transaction)
}

type EthTxJson struct {
	Type       hexutil.Uint      `json:"type"`
	Hash       common.Hash       `json:"hash"`
	Nonce      hexutil.Uint64    `json:"nonce"`
	GasPrice   *hexutil.Big      `json:"gasPrice,omitempty"`
	GasTipCap  *hexutil.Big      `json:"maxPriorityFeePerGas,omitempty"`
	GasFeeCap  *hexutil.Big      `json:"maxFeePerGas,omitempty"`
	Gas        hexutil.Uint64    `json:"gas"`
	To         *common.Address   `json:"to"`
	Value      hexutil.Big       `json:"value"`
	AccessList *types.AccessList `json:"accessList,omitempty"`
	Data       hexutil.Bytes     `json:"input"`
	V          hexutil.Big       `json:"v"`
	R          hexutil.Big       `json:"r"`
	S          hexutil.Big       `json:"s"`
	ChainID    hexutil.Uint      `json:"chainId"`
	Sender     common.Address    `json:"from"`
}
