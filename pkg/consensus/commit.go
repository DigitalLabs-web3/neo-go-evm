package consensus

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/dbft/payload"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
)

// commit represents dBFT Commit message.
type commit struct {
	signature [signatureSize]byte
}

// signatureSize is an rfc6989 signature size in bytes
// without leading byte (0x04, uncompressed).
const signatureSize = 64

var _ payload.Commit = (*commit)(nil)

// EncodeBinary implements io.Serializable interface.
func (c *commit) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(c.signature[:])
}

// DecodeBinary implements io.Serializable interface.
func (c *commit) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(c.signature[:])
}

// Signature implements payload.Commit interface.
func (c commit) Signature() []byte { return c.signature[:] }

// SetSignature implements payload.Commit interface.
func (c *commit) SetSignature(signature []byte) {
	copy(c.signature[:], signature)
}
