package state

import (
	"encoding/hex"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
	contract := Contract{
		Address:  common.HexToAddress("01"),
		CodeHash: common.HexToHash("02"),
		Code:     []byte{1, 2, 3},
	}
	b, err := io.ToByteArray(&contract)
	require.NoError(t, err)
	t.Log(hex.EncodeToString(b))
	c := new(Contract)
	err = io.FromByteArray(c, b)
	require.NoError(t, err)
}
