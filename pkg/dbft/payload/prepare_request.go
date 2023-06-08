package payload

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// PrepareRequest represents dBFT PrepareRequest message.
type PrepareRequest interface {
	// Timestamp returns this message's timestamp.
	Timestamp() uint64
	// SetTimestamp sets timestamp of this message.
	SetTimestamp(ts uint64)

	// Nonce is a random nonce.
	Nonce() uint64
	// SetNonce sets Nonce.
	SetNonce(nonce uint64)

	// TransactionHashes returns hashes of all transaction in a proposed block.
	TransactionHashes() []common.Hash
	// SetTransactionHashes sets transaction's hashes.
	SetTransactionHashes(hs []common.Hash)

	// NextConsensus returns hash which is based on which validators will
	// try to agree on a block in the current epoch.
	NextConsensus() common.Address
	// SetNextConsensus sets next consensus field.
	SetNextConsensus(nc common.Address)
}

type prepareRequest struct {
	transactionHashes []common.Hash
	nonce             uint64
	timestamp         uint32
	nextConsensus     common.Address
}

var _ PrepareRequest = (*prepareRequest)(nil)

// EncodeBinary implements io.Serializable interface.
func (p prepareRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(p.timestamp)
	w.WriteU64LE(p.nonce)
	w.WriteBytes(p.nextConsensus[:])
	w.WriteVarUint(uint64(len(p.transactionHashes)))
	for _, hash := range p.transactionHashes {
		w.WriteBytes(hash[:])
	}
}

// DecodeBinary implements io.Serializable interface.
func (p *prepareRequest) DecodeBinary(r *io.BinReader) {
	p.timestamp = r.ReadU32LE()
	p.nonce = r.ReadU64LE()
	r.ReadBytes(p.nextConsensus[:])
	count := r.ReadVarUint()
	p.transactionHashes = make([]common.Hash, count)
	for i := uint64(0); i < count; i++ {
		r.ReadBytes(p.transactionHashes[i][:])
	}
}

// Timestamp implements PrepareRequest interface.
func (p prepareRequest) Timestamp() uint64 {
	return secToNanoSec(p.timestamp)
}

// SetTimestamp implements PrepareRequest interface.
func (p *prepareRequest) SetTimestamp(ts uint64) {
	p.timestamp = nanoSecToSec(ts)
}

// Nonce implements PrepareRequest interface.
func (p prepareRequest) Nonce() uint64 {
	return p.nonce
}

// SetNonce implements PrepareRequest interface.
func (p *prepareRequest) SetNonce(nonce uint64) {
	p.nonce = nonce
}

// TransactionHashes implements PrepareRequest interface.
func (p prepareRequest) TransactionHashes() []common.Hash {
	return p.transactionHashes
}

// SetTransactionHashes implements PrepareRequest interface.
func (p *prepareRequest) SetTransactionHashes(hs []common.Hash) {
	p.transactionHashes = hs
}

// NextConsensus implements PrepareRequest interface.
func (p prepareRequest) NextConsensus() common.Address {
	return p.nextConsensus
}

// SetNextConsensus implements PrepareRequest interface.
func (p *prepareRequest) SetNextConsensus(nc common.Address) {
	p.nextConsensus = nc
}
