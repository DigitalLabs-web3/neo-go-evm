package transaction

import (
	"encoding/json"
	"errors"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	nio "github.com/neo-ngd/neo-go/pkg/io"
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
	t.ChainID, t.Sender, err = deriveSigned(tx)
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
	return nil
}

func (t *EthTx) Verify(chainId uint64) (err error) {
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

func (t *EthTx) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &t.Transaction)
}

func (t *EthTx) DecodeRLP(s *rlp.Stream) error {
	return s.Decode(&t.Transaction)
}

func (t *EthTx) EncodeBinary(w *nio.BinWriter) {
	err := rlp.Encode(w, t)
	w.Err = err
}

func (t *EthTx) DecodeBinary(r *nio.BinReader) {
	var err error
	defer func() {
		r.Err = err
	}()
	err = rlp.Decode(r, t)
	if err != nil {
		return
	}
	t.ChainID, t.Sender, err = deriveSigned(&t.Transaction)
	if err != nil {
		return
	}
}

func (t *EthTx) MarshalJSON() ([]byte, error) {
	v, r, s := t.Transaction.RawSignatureValues()
	tx := &ethTxJson{
		Nonce:    hexutil.Uint64(t.Nonce()),
		GasPrice: hexutil.Big(*t.GasPrice()),
		Gas:      hexutil.Uint64(t.Gas()),
		To:       t.To(),
		Value:    hexutil.Big(*t.Value()),
		Data:     hexutil.Bytes(t.Data()),
		V:        hexutil.Big(*v),
		R:        hexutil.Big(*r),
		S:        hexutil.Big(*s),
		ChainID:  hexutil.Uint(t.ChainID),
		Sender:   t.Sender,
	}
	if t.Transaction.Type() == types.DynamicFeeTxType {
		tx.GasFeeCap = (*hexutil.Big)(t.Transaction.GasFeeCap())
		tx.GasTipCap = (*hexutil.Big)(t.Transaction.GasTipCap())
	}
	if t.Transaction.Type() != types.LegacyTxType {
		al := t.Transaction.AccessList()
		tx.AccessList = &al
	}
	return json.Marshal(tx)
}

func (t *EthTx) UnmarshalJSON(data []byte) error {
	tx := new(ethTxJson)
	err := json.Unmarshal(data, tx)
	if err != nil {
		return err
	}
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

type ethTxJson struct {
	Nonce      hexutil.Uint64    `json:"nonce"`
	GasPrice   hexutil.Big       `json:"gasPrice"`
	GasTipCap  *hexutil.Big      `json:"gasTipCap,omitempty"`
	GasFeeCap  *hexutil.Big      `json:"gasFeeCap,omitempty"`
	Gas        hexutil.Uint64    `json:"gas"`
	To         *common.Address   `json:"to,omitempty"`
	Value      hexutil.Big       `json:"value"`
	AccessList *types.AccessList `json:"accessList,omitempty"`
	Data       hexutil.Bytes     `json:"data"`
	V          hexutil.Big       `json:"V"`
	R          hexutil.Big       `json:"R"`
	S          hexutil.Big       `json:"S"`
	ChainID    hexutil.Uint      `json:"chainId"`
	Sender     common.Address    `json:"sender"`
}
