package payload

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// PrepareResponse represents dBFT PrepareResponse message.
type PrepareResponse interface {
	// PreparationHash returns the hash of PrepareRequest payload
	// for this epoch.
	PreparationHash() common.Hash
	// SetPreparationHash sets preparations hash.
	SetPreparationHash(h common.Hash)
}

type prepareResponse struct {
	preparationHash common.Hash
}

var _ PrepareResponse = (*prepareResponse)(nil)

// EncodeBinary implements io.Serializable interface.
func (p prepareResponse) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(p.preparationHash[:])
}

// DecodeBinary implements io.Serializable interface.
func (p *prepareResponse) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(p.preparationHash[:])
}

// PreparationHash implements PrepareResponse interface.
func (p *prepareResponse) PreparationHash() common.Hash {
	return p.preparationHash
}

// SetPreparationHash implements PrepareResponse interface.
func (p *prepareResponse) SetPreparationHash(h common.Hash) {
	p.preparationHash = h
}
