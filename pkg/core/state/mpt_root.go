package state

import (
	"errors"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// MPTRoot represents storage state root together with sign info.
type MPTRoot struct {
	Version byte                `json:"version"`
	Index   uint32              `json:"index"`
	Root    common.Hash         `json:"roothash"`
	Witness transaction.Witness `json:"witnesses"`
}

// Hash returns hash of s.
func (s *MPTRoot) Hash() common.Hash {
	buf := io.NewBufBinWriter()
	s.EncodeBinaryUnsigned(buf.BinWriter)
	return hash.Sha256(buf.Bytes())
}

// DecodeBinaryUnsigned decodes hashable part of state root.
func (s *MPTRoot) DecodeBinaryUnsigned(r *io.BinReader) {
	s.Version = r.ReadB()
	s.Index = r.ReadU32LE()
	r.ReadBytes(s.Root[:])
}

// EncodeBinaryUnsigned encodes hashable part of state root..
func (s *MPTRoot) EncodeBinaryUnsigned(w *io.BinWriter) {
	w.WriteB(s.Version)
	w.WriteU32LE(s.Index)
	w.WriteBytes(s.Root[:])
}

// DecodeBinary implements io.Serializable.
func (s *MPTRoot) DecodeBinary(r *io.BinReader) {
	s.DecodeBinaryUnsigned(r)
	witnessCount := r.ReadVarUint()
	if r.Err == nil && witnessCount != 1 {
		r.Err = errors.New("wrong witness count")
		return
	}
	s.Witness.DecodeBinary(r)
}

// EncodeBinary implements io.Serializable.
func (s *MPTRoot) EncodeBinary(w *io.BinWriter) {
	s.EncodeBinaryUnsigned(w)
	w.WriteVarUint(1)
	s.Witness.EncodeBinary(w)
}
