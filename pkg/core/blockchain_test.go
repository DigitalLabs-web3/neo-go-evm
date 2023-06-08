package core

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/wallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
)

func TestSign1(t *testing.T) {
	s := "f868800182d6d8946cb3e9d55dc87d4586bd2e558a1ee46a94e300b4880de0b6b3a764000080818da0ccd99c35f99317a094d99a8b677acb495dda3402e8aa45f91c524dea9aeb4c7ba02e5fffeedca5c7e4bbc7f36f16e94d1f85daf0ed0cd036187aa9c564708ce220"
	d, err := hex.DecodeString(s)
	assert.NoError(t, err)
	ltx := &types.LegacyTx{}
	err = rlp.DecodeBytes(d, ltx)
	assert.NoError(t, err)
	tx := transaction.NewTx(ltx)
	err = tx.Verify(53)
	assert.NoError(t, err)
	t.Log(tx.From())
	t.Log(tx.Hash())
	pkbs, err := hex.DecodeString("655119e8830ed7816c91a6ce8138560854687e2c78074d065becc66fe5a65f6b")
	assert.NoError(t, err)
	pk, err := keys.NewPrivateKeyFromBytes(pkbs)
	assert.NoError(t, err)
	acc := wallet.NewAccountFromPrivateKey(pk)
	ltx0 := &types.LegacyTx{
		Nonce:    ltx.Nonce,
		GasPrice: ltx.GasPrice,
		Gas:      ltx.Gas,
		Value:    ltx.Value,
		To:       ltx.To,
		Data:     ltx.Data,
	}
	tx = transaction.NewTx(ltx0)
	err = acc.SignTx(53, tx)
	assert.NoError(t, err)
	err = tx.Verify(53)
	assert.NoError(t, err)
	t.Log(tx.From())
}

func TestSign2(t *testing.T) {
	ltx := &types.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1),
		Gas:      1,
		To:       &common.Address{},
		Value:    big.NewInt(1),
		Data:     []byte{},
	}
	tx := transaction.NewTx(ltx)
	hash := tx.Hash()
	pkbs, err := hex.DecodeString("655119e8830ed7816c91a6ce8138560854687e2c78074d065becc66fe5a65f6b")
	assert.NoError(t, err)
	pk, err := keys.NewPrivateKeyFromBytes(pkbs)
	assert.NoError(t, err)
	t.Log(hex.EncodeToString(pk.PublicKey().UncompressedBytes()))
	sig, err := crypto.Sign(hash.Bytes(), &pk.PrivateKey)
	assert.NoError(t, err)
	pubk, err := crypto.Ecrecover(hash.Bytes(), sig)
	assert.NoError(t, err)
	t.Log(hex.EncodeToString(pubk))
	t.Log(crypto.PubkeyToAddress(pk.PrivateKey.PublicKey))
	signer := types.NewEIP155Signer(big.NewInt(53))
	r, s, v, err := signer.SignatureValues(types.NewTx(ltx), sig)
	assert.NoError(t, err)
	ltx.R = r
	ltx.S = s
	ltx.V = v
	addr, err := signer.Sender(types.NewTx(ltx))
	assert.NoError(t, err)
	t.Log(addr)
}
