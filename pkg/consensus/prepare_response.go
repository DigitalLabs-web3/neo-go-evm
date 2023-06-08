package consensus

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/dbft/payload"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// prepareResponse represents dBFT PrepareResponse message.
type prepareResponse struct {
	preparationHash common.Hash
}

var _ payload.PrepareResponse = (*prepareResponse)(nil)

// EncodeBinary implements io.Serializable interface.
func (p *prepareResponse) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(p.preparationHash[:])
}

// DecodeBinary implements io.Serializable interface.
func (p *prepareResponse) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(p.preparationHash[:])
}

// PreparationHash implements payload.PrepareResponse interface.
func (p *prepareResponse) PreparationHash() common.Hash { return p.preparationHash }

// SetPreparationHash implements payload.PrepareResponse interface.
func (p *prepareResponse) SetPreparationHash(h common.Hash) { p.preparationHash = h }
