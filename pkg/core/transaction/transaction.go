package transaction

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"sync/atomic"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	nio "github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	ErrInvalidTxType    = errors.New("invalid tx type")
	ErrTipVeryHigh      = errors.New("max priority fee per gas higher than 2^256-1")
	ErrFeeCapVeryHigh   = errors.New("max fee per gas higher than 2^256-1")
	ErrTipAboveFeeCap   = errors.New("max priority fee per gas higher than max fee per gas")
	ErrValueVeryHigh    = errors.New("value higher than 2^256-1")
	ErrGasPriceVeryHigh = errors.New("gas price higher than 2^256-1")
	ErrNegativeValue    = errors.New("negative value")
	ErrZeroFromAddress  = errors.New("zero from address")
	ErrUnsupportType    = errors.New("unsupport tx type")
	ErrInvalidChainID   = errors.New("invalid chainId")
)

const (
	MaxScriptLength    = math.MaxUint16
	MaxTransactionSize = 102400
)

type Transaction struct {
	types.Transaction
	ChainID uint64
	Trimmed bool
	Sender  common.Address

	hash atomic.Value
}

func NewTrimmedTX(hash common.Hash) *Transaction {
	t := &Transaction{
		Trimmed: true,
	}
	t.hash.Store(hash)
	return t
}

func NewTx(tx *types.Transaction) (*Transaction, error) {
	t := &Transaction{
		Transaction: *tx,
	}
	var err error
	t.ChainID, t.Sender, err = deriveSigned(tx)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Transaction) Hash() common.Hash {
	if t.Trimmed {
		return t.hash.Load().(common.Hash)
	}
	return t.Transaction.Hash()
}

func (t *Transaction) From() common.Address {
	return t.Sender
}

func (t *Transaction) Size() int {
	return int(t.Transaction.Size())
}

func (t *Transaction) Bytes() ([]byte, error) {
	return io.ToByteArray(t)
}

func (t *Transaction) FeePerByte() uint64 {
	return t.Gas() / uint64(t.Size())
}

func (t Transaction) SignHash(chainId uint64) common.Hash {
	signer := types.NewEIP155Signer(big.NewInt(int64(chainId)))
	return signer.Hash(&t.Transaction)
}

func (t *Transaction) WithSignature(chainId uint64, sig []byte) error {
	signer := types.NewEIP155Signer(big.NewInt(int64(chainId)))
	tx, err := t.Transaction.WithSignature(signer, sig)
	t.Transaction = *tx
	return err
}

func (t *Transaction) IsValid() error {
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

func (t *Transaction) Verify(chainId uint64) (err error) {
	if t.ChainID == 0 && t.Sender == (common.Address{}) {
		t.ChainID, t.Sender, err = deriveSigned(&t.Transaction)
		if err != nil {
			return
		}
	}
	if t.ChainID != chainId {
		return ErrInvalidChainID
	}
	return nil
}

func (t *Transaction) EncodeBinary(w *nio.BinWriter) {
	var err error
	defer func() {
		w.Err = err
	}()
	b, err := t.MarshalBinary()
	if err != nil {
		return
	}
	w.WriteVarBytes(b)
}

func (t *Transaction) DecodeBinary(r *nio.BinReader) {
	var err error
	defer func() {
		r.Err = err
	}()
	b := r.ReadVarBytes(MaxTransactionSize)
	if r.Err != nil {
		return
	}
	err = t.Transaction.UnmarshalBinary(b)
	if err != nil {
		return
	}
	t.ChainID, t.Sender, err = deriveSigned(&t.Transaction)
	if err != nil {
		return
	}
}

func (t Transaction) MarshalJSON() ([]byte, error) {
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

func (t *Transaction) UnmarshalJSON(data []byte) error {
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

func deriveSigned(t *types.Transaction) (chainId uint64, sender common.Address, err error) {
	bigChainId := t.ChainId()
	if !bigChainId.IsUint64() {
		err = errors.New("ChainId is not uint64")
		return
	}
	chainId = bigChainId.Uint64()
	sender, err = deriveSender(t, chainId)
	if err != nil {
		return
	}
	return
}

func deriveSender(t *types.Transaction, chainId uint64) (common.Address, error) {
	signer := types.NewLondonSigner(big.NewInt(int64(chainId)))
	return signer.Sender(t)
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
