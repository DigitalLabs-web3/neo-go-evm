package payload

import (
	"errors"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// Maximum inventory hashes number is limited to 500.
const (
	MaxHashesCount = 500
)

// GetBlocks contains getblocks message payload fields.
type GetBlocks struct {
	// Hash of the latest block that node requests.
	HashStart common.Hash
	Count     int16
}

// NewGetBlocks returns a pointer to a GetBlocks object.
func NewGetBlocks(start common.Hash, count int16) *GetBlocks {
	return &GetBlocks{
		HashStart: start,
		Count:     count,
	}
}

// DecodeBinary implements Serializable interface.
func (p *GetBlocks) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(p.HashStart[:])
	p.Count = int16(br.ReadU16LE())
	if p.Count < -1 || p.Count == 0 {
		br.Err = errors.New("invalid count")
	}
}

// EncodeBinary implements Serializable interface.
func (p *GetBlocks) EncodeBinary(bw *io.BinWriter) {
	bw.WriteBytes(p.HashStart[:])
	bw.WriteU16LE(uint16(p.Count))
}
