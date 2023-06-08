package transaction

import (
	"math/big"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
)

func TestDecode(t *testing.T) {
	hex := "0x0100000000000000043b9aca00785d020000000000c20f7284c317f0b1d747aff294dbbd38a2347aad1400000000000000000000000000000000000000e40003010100406bd84ddf171651c38132d4bda4d393cc9b7e17f350360bc6bddbc0fc39eca564a9cea57f4c4461ecbbb0e1b385f14908792c3dcaab3f6bafb65ccbc506b6956a2200028c43c918440067b4b71b601871752a0b549092ff9aa9a7bc46f5244684f30ea5"
	tx := &NeoTx{}
	b, err := hexutil.Decode(hex)
	assert.NoError(t, err)
	err = io.FromByteArray(tx, b)
	assert.NoError(t, err)
}

func TestEncodeDecode(t *testing.T) {
	txx := &NeoTx{
		Nonce:    0,
		GasPrice: big.NewInt(1),
		Gas:      1,
		From:     common.HexToAddress("0xc20f7284c317f0b1d747aff294dbbd38a2347aad"),
		To:       &common.Address{},
		Value:    big.NewInt(0),
		Data:     []byte{1},
		Witness: Witness{
			VerificationScript: []byte{2},
			InvocationScript:   []byte{3},
		},
	}
	h1 := txx.Hash()
	b, err := io.ToByteArray(txx)
	assert.NoError(t, err)
	tx := &NeoTx{}
	err = io.FromByteArray(tx, b)
	assert.NoError(t, err)
	assert.Equal(t, h1, tx.Hash())
}
