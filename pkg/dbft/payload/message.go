package payload

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

type (
	// ConsensusPayload is a generic payload type which is exchanged
	// between the nodes.
	ConsensusPayload interface {
		consensusMessage

		// ValidatorIndex returns index of validator from which
		// payload was originated from.
		ValidatorIndex() uint16

		// SetValidator index sets validator index.
		SetValidatorIndex(i uint16)

		Height() uint32
		SetHeight(h uint32)

		// Hash returns 32-byte checksum of the payload.
		Hash() common.Hash
	}

	// Payload represents minimal payload containing all necessary fields.
	Payload struct {
		message

		version        uint32
		validatorIndex uint16
		prevHash       common.Hash
		height         uint32

		hash *common.Hash
	}
)

var _ ConsensusPayload = (*Payload)(nil)

// EncodeBinary implements io.Serializable interface.
func (p Payload) EncodeBinary(w *io.BinWriter) {
	ww := io.NewBufBinWriter()
	p.message.EncodeBinary(ww.BinWriter)
	data := ww.Bytes()

	w.WriteU32LE(p.version)
	w.WriteBytes(p.prevHash[:])
	w.WriteU32LE(p.height)
	w.WriteU16LE(p.validatorIndex)
	w.WriteVarBytes(data)
}

// DecodeBinary implements io.Serializable interface.
func (p *Payload) DecodeBinary(r *io.BinReader) {
	p.version = r.ReadU32LE()
	r.ReadBytes(p.prevHash[:])
	p.height = r.ReadU32LE()
	p.validatorIndex = r.ReadU16LE()

	data := r.ReadVarBytes()
	rr := io.NewBinReaderFromBuf(data)
	p.message.DecodeBinary(rr)
}

// MarshalUnsigned implements ConsensusPayload interface.
func (p Payload) MarshalUnsigned() []byte {
	w := io.NewBufBinWriter()
	p.EncodeBinary(w.BinWriter)

	return w.Bytes()
}

// UnmarshalUnsigned implements ConsensusPayload interface.
func (p *Payload) UnmarshalUnsigned(data []byte) error {
	r := io.NewBinReaderFromBuf(data)
	p.DecodeBinary(r)

	return r.Err
}

// Hash implements ConsensusPayload interface.
func (p *Payload) Hash() common.Hash {
	if p.hash != nil {
		return *p.hash
	}

	data := p.MarshalUnsigned()

	return hash.Keccak256(data)
}

// Version implements ConsensusPayload interface.
func (p Payload) Version() uint32 {
	return p.version
}

// SetVersion implements ConsensusPayload interface.
func (p *Payload) SetVersion(v uint32) {
	p.version = v
}

// ValidatorIndex implements ConsensusPayload interface.
func (p Payload) ValidatorIndex() uint16 {
	return p.validatorIndex
}

// SetValidatorIndex implements ConsensusPayload interface.
func (p *Payload) SetValidatorIndex(i uint16) {
	p.validatorIndex = i
}

// PrevHash implements ConsensusPayload interface.
func (p Payload) PrevHash() common.Hash {
	return p.prevHash
}

// SetPrevHash implements ConsensusPayload interface.
func (p *Payload) SetPrevHash(h common.Hash) {
	p.prevHash = h
}

// Height implements ConsensusPayload interface.
func (p Payload) Height() uint32 {
	return p.height
}

// SetHeight implements ConsensusPayload interface.
func (p *Payload) SetHeight(h uint32) {
	p.height = h
}
