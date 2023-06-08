package consensus

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/dbft/payload"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// prepareRequest represents dBFT prepareRequest message.
type prepareRequest struct {
	version           uint32
	prevHash          common.Hash
	timestamp         uint64
	nonce             uint64
	nextConsensus     common.Address
	transactionHashes []common.Hash
	stateRootEnabled  bool
	stateRoot         common.Hash
}

var _ payload.PrepareRequest = (*prepareRequest)(nil)

// EncodeBinary implements io.Serializable interface.
func (p *prepareRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(p.version)
	w.WriteBytes(p.prevHash[:])
	w.WriteU64LE(p.timestamp)
	w.WriteU64LE(p.nonce)
	w.WriteBytes(p.nextConsensus[:])
	w.WriteVarUint(uint64(len(p.transactionHashes)))
	for _, h := range p.transactionHashes {
		w.WriteBytes(h[:])
	}
	if p.stateRootEnabled {
		w.WriteBytes(p.stateRoot[:])
	}
}

// DecodeBinary implements io.Serializable interface.
func (p *prepareRequest) DecodeBinary(r *io.BinReader) {
	p.version = r.ReadU32LE()
	r.ReadBytes(p.prevHash[:])
	p.timestamp = r.ReadU64LE()
	p.nonce = r.ReadU64LE()
	r.ReadBytes(p.nextConsensus[:])
	count := r.ReadVarUint()
	if count > block.MaxTransactionsPerBlock {
		count = block.MaxTransactionsPerBlock
	}
	p.transactionHashes = make([]common.Hash, count)
	for i := uint64(0); i < count; i++ {
		r.ReadBytes(p.transactionHashes[i][:])
	}
	if p.stateRootEnabled {
		r.ReadBytes(p.stateRoot[:])
	}
}

// Version implements payload.PrepareRequest interface.
func (p prepareRequest) Version() uint32 {
	return p.version
}

// SetVersion implements payload.PrepareRequest interface.
func (p *prepareRequest) SetVersion(v uint32) {
	p.version = v
}

// PrevHash implements payload.PrepareRequest interface.
func (p prepareRequest) PrevHash() common.Hash {
	return p.prevHash
}

// SetPrevHash implements payload.PrepareRequest interface.
func (p *prepareRequest) SetPrevHash(h common.Hash) {
	p.prevHash = h
}

// Timestamp implements payload.PrepareRequest interface.
func (p *prepareRequest) Timestamp() uint64 { return p.timestamp * nsInMs }

// SetTimestamp implements payload.PrepareRequest interface.
func (p *prepareRequest) SetTimestamp(ts uint64) { p.timestamp = ts / nsInMs }

// Nonce implements payload.PrepareRequest interface.
func (p *prepareRequest) Nonce() uint64 { return p.nonce }

// SetNonce implements payload.PrepareRequest interface.
func (p *prepareRequest) SetNonce(nonce uint64) { p.nonce = nonce }

// TransactionHashes implements payload.PrepareRequest interface.
func (p *prepareRequest) TransactionHashes() []common.Hash { return p.transactionHashes }

// SetTransactionHashes implements payload.PrepareRequest interface.
func (p *prepareRequest) SetTransactionHashes(hs []common.Hash) { p.transactionHashes = hs }

// NextConsensus implements payload.PrepareRequest interface.
func (p *prepareRequest) NextConsensus() common.Address {
	return p.nextConsensus
}

// SetNextConsensus implements payload.PrepareRequest interface.
func (p *prepareRequest) SetNextConsensus(address common.Address) {
	p.nextConsensus = address
}
