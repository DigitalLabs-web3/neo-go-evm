package transaction

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	nio "github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
)

func TestSize(t *testing.T) {
	tx := &NeoTx{
		Nonce: 0,
		Data:  []byte{},
	}
	d, _ := nio.ToByteArray(tx)
	assert.Equal(t, 42, len(d))
	assert.Equal(t, 42, tx.Size())
}

func TestEthTxDecode(t *testing.T) {
	s := "f868800182d6d8946cb3e9d55dc87d4586bd2e558a1ee46a94e300b4880de0b6b3a764000080818da0ccd99c35f99317a094d99a8b677acb495dda3402e8aa45f91c524dea9aeb4c7ba02e5fffeedca5c7e4bbc7f36f16e94d1f85daf0ed0cd036187aa9c564708ce220"
	ds, err := hex.DecodeString(s)
	assert.NoError(t, err)
	tx := &types.LegacyTx{}
	err = rlp.DecodeBytes(ds, tx)
	assert.NoError(t, err)
	d, err := json.Marshal(tx)
	assert.NoError(t, err)
	t.Log(string(d))
	txx := &types.LegacyTx{}
	bss := append(ds, 1)
	reader := nio.NewBinReaderFromBuf(bss)
	err = rlp.Decode(reader, txx)
	assert.NoError(t, err)
	b := reader.ReadB()
	assert.Equal(t, byte(1), b)
}

func TestHexutil(t *testing.T) {
	hex := "0xd6d8"
	n, err := hexutil.DecodeUint64(hex)
	assert.NoError(t, err)
	t.Log(n)
}

func TestNetFee(t *testing.T) {
	s := "f868800182d6d8946cb3e9d55dc87d4586bd2e558a1ee46a94e300b4880de0b6b3a764000080818da0ccd99c35f99317a094d99a8b677acb495dda3402e8aa45f91c524dea9aeb4c7ba02e5fffeedca5c7e4bbc7f36f16e94d1f85daf0ed0cd036187aa9c564708ce220"
	d, err := hex.DecodeString(s)
	assert.NoError(t, err)
	ltx := &types.LegacyTx{}
	err = rlp.DecodeBytes(d, ltx)
	assert.NoError(t, err)
	ethtx, err := NewEthTx(types.NewTx(ltx))
	assert.NoError(t, err)
	tx := NewTx(ethtx)
	actual := uint64(tx.Size()) * 1
	cal := CalculateNetworkFee(tx, 1)
	siglen := RlpSize(ltx.R) + RlpSize(ltx.S) + RlpSize(ltx.V)
	t.Log(siglen)
	t.Log(RlpSize(ltx))
	assert.NoError(t, err)
	t.Logf("%d %d\n", actual, cal)
	ltx.R = nil
	ltx.S = nil
	ltx.V = nil
	t.Log(RlpSize(ltx))
}

func TestEncodeLegacy(t *testing.T) {
	tx := NewTx(&EthTx{
		Transaction: *types.NewTx(&types.LegacyTx{}),
	})
	b, err := nio.ToByteArray(tx)
	assert.NoError(t, err)
	txx := &Transaction{}
	err = nio.FromByteArray(txx, b)
	assert.Error(t, err)
}

func TestCancel(t *testing.T) {
	hex1 := `f86d80843b9aca00830186a0946cb3e9d55dc87d4586bd2e558a1ee46a94e300b4880de0b6b3a764000080818ea02ec25d5b4ef673d4fc95e910c8b6861db9d5136bf65c31b372cc95c3943eb309a0146e92873ad2e74150e830fa4c36ace7d11f1a27897a886b02633a1af0572b71`
	hex2 := `f865808477359400830186a094d751051783f45346be80d5fb2f7483c284d8377f8080818ea0e797d15f92c19d5f41e43c386e45dc54fa0156758efe5c158b2e67c818e41351a00a1c02f1eac66cbbdc81c1be0c8ee82a8b0434796c92ae15209ddaf6771dce92`
	t1 := &types.LegacyTx{}
	b1, err := hex.DecodeString(hex1)
	assert.NoError(t, err)
	err = rlp.DecodeBytes(b1, t1)
	assert.NoError(t, err)
	t2 := &types.LegacyTx{}
	b2, err := hex.DecodeString(hex2)
	assert.NoError(t, err)
	err = rlp.DecodeBytes(b2, t2)
	assert.NoError(t, err)
	assert.Equal(t, t1.Nonce, t2.Nonce)
	etx1, err := NewEthTx(types.NewTx(t1))
	assert.NoError(t, err)
	b1, err = json.Marshal(etx1)
	assert.NoError(t, err)
	fmt.Println(string(b1))
	etx2, err := NewEthTx(types.NewTx(t2))
	assert.NoError(t, err)
	b2, err = json.Marshal(etx2)
	assert.NoError(t, err)
	fmt.Println(string(b2))
}

func TestJson(t *testing.T) {
	ltx := &types.LegacyTx{
		Nonce: 1,
		Data:  []byte{1},
		To:    &common.Address{},
	}
	tx := types.NewTx(ltx)
	etx := &EthTx{
		Transaction: *tx,
	}
	txx := NewTx(etx)
	b, err := json.Marshal(txx)
	assert.NoError(t, err)
	t.Log(string(b))
	txxx := new(Transaction)
	err = json.Unmarshal(b, txxx)
	assert.NoError(t, err)
	assert.Equal(t, EthTxType, txxx.Type)
	assert.Equal(t, types.LegacyTxType, int(txxx.EthTx.Type()))
}
