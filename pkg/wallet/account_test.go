package wallet

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestAddressJSON(t *testing.T) {
	addr := common.HexToAddress("0xb2f58f5d4b6756e8c98d31966619ab084a579a89")
	t.Log(addr)
	d, err := json.Marshal(addr)
	assert.NoError(t, err)
	t.Log(string(d))
}

func TestPrivateKeyToAddress(t *testing.T) {
	hexPrivateKey := "655119e8830ed7816c91a6ce8138560854687e2c78074d065becc66fe5a65f6b"
	pk, err := crypto.HexToECDSA(hexPrivateKey)
	assert.NoError(t, err)
	t.Log(crypto.PubkeyToAddress(pk.PublicKey))
	ppk, err := keys.NewPrivateKeyFromHex(hexPrivateKey)
	assert.NoError(t, err)
	t.Log(ppk.Address())
}

func TestBigInt(t *testing.T) {
	floor := 52000000
	wei := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)
	initial := big.NewInt(1).Mul(big.NewInt(int64(floor)), wei)
	t.Log(hex.EncodeToString(initial.Bytes()))
}

func TestOneMultiAddress(t *testing.T) {
	k, err := keys.NewPublicKeyFromString("028c43c918440067b4b71b601871752a0b549092ff9aa9a7bc46f5244684f30ea5")
	assert.NoError(t, err)
	ks := keys.PublicKeys{k}
	script, err := ks.CreateDefaultMultiSigRedeemScript()
	assert.NoError(t, err)
	t.Log(hash.Hash160(script))

	k, err = keys.NewPublicKeyFromString("03092c7fc564d67a2c589a4229c54f19358629529bc191daf1642d0a95989e3a83")
	assert.NoError(t, err)
	ks = keys.PublicKeys{k}
	script, err = ks.CreateDefaultMultiSigRedeemScript()
	assert.NoError(t, err)
	t.Log(hash.Hash160(script))
}
