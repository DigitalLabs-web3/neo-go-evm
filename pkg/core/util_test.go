package core

import (
	"encoding/hex"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestGenesisBlock(t *testing.T) {
	b, err := createGenesisBlock(&config.ProtocolConfiguration{})
	assert.NoError(t, err)
	bs, err := io.ToByteArray(b)
	assert.NoError(t, err)
	bb := &block.Block{}
	err = io.FromByteArray(bb, bs)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(bb.Transactions))
}

func TestGenesisEncode(t *testing.T) {
	b, err := hex.DecodeString("000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000065fc8857000000001dac2b7c0000000000000000003285f72d756ce0c5fd96f93512eb0ebda8b9f9d1000000")
	assert.NoError(t, err)
	blk := new(block.Block)
	err = io.FromByteArray(blk, b)
	assert.NoError(t, err)
}
