package payload

import (
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestInventoryEncode(t *testing.T) {
	hashes := []common.Hash{
		common.HexToHash("0x8228840c950c5c7402828d2f073d5973ded9f6147ae6a57767d68229531a2082"),
		common.HexToHash("0x0f795334484b66b296f6ded28162a7467fd445eb387469119c4be2247e174976"),
	}
	inv := NewInventory(ExtensibleType, hashes)
	b, err := io.ToByteArray(inv)
	assert.NoError(t, err)
	i := &Inventory{}
	err = io.FromByteArray(i, b)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(i.Hashes))
	assert.Equal(t, common.HexToHash("0x8228840c950c5c7402828d2f073d5973ded9f6147ae6a57767d68229531a2082"), i.Hashes[0])
}
