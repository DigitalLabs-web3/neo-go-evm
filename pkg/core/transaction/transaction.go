package transaction

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"sync/atomic"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	EthTxType           = byte(0)
	NeoTxType           = byte(1)
	SignatureLength     = 64
	MaxScriptLength     = math.MaxUint16
	MaxTransactionSize  = 102400
	EthLegacyBaseLength = 100
)

var (
	ErrUnsupportType = errors.New("unsupport tx type")
)

type Transaction struct {
	Type  byte
	EthTx *EthTx
	NeoTx *NeoTx

	Trimmed bool
	hash    atomic.Value
	size    atomic.Value
}

func NewTrimmedTX(hash common.Hash) *Transaction {
	t := &Transaction{
		Trimmed: true,
	}
	t.hash.Store(hash)
	return t
}

func NewTx(t interface{}) *Transaction {
	tx := &Transaction{}
	switch v := t.(type) {
	case *NeoTx:
		tx.Type = NeoTxType
		tx.NeoTx = v
	case *EthTx:
		tx.Type = EthTxType
		tx.EthTx = v
	default:
		panic("unsupport tx")
	}
	return tx
}

func NewTransactionFromBytes(b []byte) (*Transaction, error) {
	tx := &Transaction{}
	err := io.FromByteArray(tx, b)
	if err != nil {
		return nil, err
	}
	return tx, err
}

func (t *Transaction) Nonce() uint64 {
	switch t.Type {
	case EthTxType:
		return t.EthTx.Nonce()
	case NeoTxType:
		return t.NeoTx.Nonce
	default:
		panic(ErrUnsupportType)
	}
}

func (t *Transaction) To() *common.Address {
	switch t.Type {
	case EthTxType:
		return t.EthTx.To()
	case NeoTxType:
		return t.NeoTx.To
	default:
		panic(ErrUnsupportType)
	}
}

func (t *Transaction) Gas() uint64 {
	switch t.Type {
	case EthTxType:
		return t.EthTx.Gas()
	case NeoTxType:
		return t.NeoTx.Gas
	default:
		panic(ErrUnsupportType)
	}
}

func (t *Transaction) GasPrice() *big.Int {
	switch t.Type {
	case EthTxType:
		return t.EthTx.GasPrice()
	case NeoTxType:
		return t.NeoTx.GasPrice
	default:
		panic(ErrUnsupportType)
	}
}

func (t Transaction) Cost() *big.Int {
	cost := big.NewInt(0).Mul(big.NewInt(int64(t.Gas())), t.GasPrice())
	return big.NewInt(0).Add(t.Value(), cost)
}

func (t *Transaction) Value() *big.Int {
	switch t.Type {
	case EthTxType:
		return t.EthTx.Value()
	case NeoTxType:
		return t.NeoTx.Value
	default:
		panic(ErrUnsupportType)
	}
}

func (t *Transaction) Data() []byte {
	switch t.Type {
	case EthTxType:
		return t.EthTx.Data()
	case NeoTxType:
		return t.NeoTx.Data
	default:
		panic(ErrUnsupportType)
	}
}

func (t *Transaction) Size() int {
	if size := t.size.Load(); size != nil {
		return size.(int)
	}
	var size int
	switch t.Type {
	case EthTxType:
		size = int(t.EthTx.Size())
	case NeoTxType:
		size = t.NeoTx.Size()
	default:
		panic(ErrUnsupportType)
	}
	t.size.Store(size)
	return size
}

func (t *Transaction) From() common.Address {
	switch t.Type {
	case EthTxType:
		return t.EthTx.Sender
	case NeoTxType:
		return t.NeoTx.From
	default:
		panic(ErrUnsupportType)
	}
}

func (t *Transaction) AccessList() types.AccessList {
	switch t.Type {
	case EthTxType:
		return t.EthTx.AccessList()
	default:
		return nil
	}
}

func (t *Transaction) Hash() common.Hash {
	if hash := t.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	var h common.Hash
	if t.Type == EthTxType {
		h = t.EthTx.Hash()
	} else {
		h = t.NeoTx.Hash()
	}
	t.hash.Store(h)
	return h
}

func (t Transaction) SignHash(chainId uint64) common.Hash {
	if t.Type == EthTxType {
		signer := types.NewEIP155Signer(big.NewInt(int64(chainId)))
		return signer.Hash(&t.EthTx.Transaction)
	} else {
		return t.Hash()
	}
}

func (t *Transaction) Bytes() ([]byte, error) {
	return io.ToByteArray(t)
}

func (t Transaction) FeePerByte() uint64 {
	return t.Gas() / uint64(t.Size())
}

func (t *Transaction) EncodeBinary(w *io.BinWriter) {
	w.WriteB(t.Type)
	switch t.Type {
	case EthTxType:
		t.EthTx.EncodeBinary(w)
	case NeoTxType:
		t.NeoTx.EncodeBinary(w)
	default:
		w.Err = ErrUnsupportType
	}
}

func (t *Transaction) DecodeBinary(r *io.BinReader) {
	t.Type = r.ReadB()
	switch t.Type {
	case EthTxType:
		inner := new(EthTx)
		inner.DecodeBinary(r)
		t.EthTx = inner
	case NeoTxType:
		inner := new(NeoTx)
		inner.DecodeBinary(r)
		t.NeoTx = inner
	default:
		r.Err = ErrUnsupportType
	}
}

func (t *Transaction) Verify(chainId uint64) error {
	switch t.Type {
	case EthTxType:
		return t.EthTx.Verify(chainId)
	case NeoTxType:
		if t.NeoTx.From != t.NeoTx.Witness.Address() {
			return ErrWitnessUnmatch
		}
		return t.NeoTx.Witness.VerifyHashable(chainId, t.NeoTx)
	default:
		return ErrUnsupportType
	}
}

func (t *Transaction) WithSignature(chainId uint64, sig []byte) error {
	switch t.Type {
	case EthTxType:
		return t.EthTx.WithSignature(chainId, sig)
	default:
		return ErrUnsupportType
	}
}

func (t *Transaction) WithWitness(witness Witness) error {
	if t.Type != NeoTxType {
		return ErrUnsupportType
	}
	t.NeoTx.Witness = witness
	return nil
}

func (t *Transaction) UnmarshalJSON(b []byte) error {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}
	if _, ok := tmp["witness"]; ok {
		tx := new(NeoTx)
		err = json.Unmarshal(b, tx)
		if err != nil {
			return err
		}
		t.Type = NeoTxType
		t.NeoTx = tx
		return nil
	}
	if _, ok := tmp["type"]; ok {
		tx := new(EthTx)
		err = json.Unmarshal(b, tx)
		if err != nil {
			return err
		}
		t.Type = EthTxType
		t.EthTx = tx
		return nil
	}
	return ErrUnsupportType
}

func (t Transaction) MarshalJSON() ([]byte, error) {
	if t.Trimmed {
		return json.Marshal(t.Hash())
	}
	switch t.Type {
	case EthTxType:
		return json.Marshal(t.EthTx)
	case NeoTxType:
		return json.Marshal(t.NeoTx)
	default:
		return nil, ErrUnsupportType
	}
}

var (
	ErrInvalidTxType    = errors.New("invalid tx type")
	ErrTipVeryHigh      = errors.New("max priority fee per gas higher than 2^256-1")
	ErrFeeCapVeryHigh   = errors.New("max fee per gas higher than 2^256-1")
	ErrTipAboveFeeCap   = errors.New("max priority fee per gas higher than max fee per gas")
	ErrValueVeryHigh    = errors.New("value higher than 2^256-1")
	ErrGasPriceVeryHigh = errors.New("gas price higher than 2^256-1")
)

func (t Transaction) IsValid() error {
	switch t.Type {
	case EthTxType:
		return t.EthTx.IsValid()
	case NeoTxType:
		return t.NeoTx.isValid()
	default:
		return ErrInvalidTxType
	}
}
