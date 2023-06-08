package payload

import (
	"errors"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// MerkleBlock represents a merkle block packet payload.
type MerkleBlock struct {
	*block.Header
	TxCount int
	Hashes  []common.Hash
	Flags   []byte
}

// DecodeBinary implements Serializable interface.
func (m *MerkleBlock) DecodeBinary(br *io.BinReader) {
	m.Header = &block.Header{}
	m.Header.DecodeBinary(br)

	txCount := int(br.ReadVarUint())
	if txCount > block.MaxTransactionsPerBlock {
		br.Err = block.ErrMaxContentsPerBlock
		return
	}
	m.TxCount = txCount
	count := br.ReadVarUint()
	if uint64(txCount) != count {
		br.Err = errors.New("invalid tx count")
		return
	}
	m.Hashes = make([]common.Hash, count)
	for i := uint64(0); i < count; i++ {
		br.ReadBytes(m.Hashes[i][:])
	}
	m.Flags = br.ReadVarBytes((txCount + 7) / 8)
}

// EncodeBinary implements Serializable interface.
func (m *MerkleBlock) EncodeBinary(bw *io.BinWriter) {
	m.Header.EncodeBinary(bw)

	bw.WriteVarUint(uint64(m.TxCount))
	bw.WriteVarUint(uint64(len(m.Hashes)))
	for _, hash := range m.Hashes {
		bw.WriteBytes(hash[:])
	}
	bw.WriteVarBytes(m.Flags)
}
