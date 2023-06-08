package block

import (
	"errors"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/dbft/merkle"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

type (
	// base is a structure containing all
	// hashable and signable fields of the block.
	base struct {
		ConsensusData uint64
		Index         uint32
		Timestamp     uint32
		Version       uint32
		MerkleRoot    common.Hash
		PrevHash      common.Hash
		NextConsensus common.Address
	}

	// Block is a generic interface for a block used by dbft.
	Block interface {
		// Hash returns block hash.
		Hash() common.Hash

		Version() uint32
		// PrevHash returns previous block hash.
		PrevHash() common.Hash
		// MerkleRoot returns a merkle root of the transaction hashes.
		MerkleRoot() common.Hash
		// Timestamp returns block's proposal timestamp.
		Timestamp() uint64
		// Index returns block index.
		Index() uint32
		// ConsensusData is a random nonce.
		ConsensusData() uint64

		// Signature returns block's signature.
		Signature() []byte
		// Sign signs block and sets it's signature.
		Sign(key *keys.PrivateKey) error
		// Verify checks if signature is correct.
		Verify(key *keys.PublicKey, sign []byte) error

		// Transactions returns block's transaction list.
		Transactions() []Transaction
		// SetTransaction sets block's transaction list.
		SetTransactions([]Transaction)
	}

	bftBlock struct {
		base

		consensusData uint64
		transactions  []Transaction
		signature     []byte
		hash          *common.Hash
	}
)

// Version implements Block interface.
func (b bftBlock) Version() uint32 {
	return b.base.Version
}

// PrevHash implements Block interface.
func (b *bftBlock) PrevHash() common.Hash {
	return b.base.PrevHash
}

// Timestamp implements Block interface.
func (b *bftBlock) Timestamp() uint64 {
	return uint64(b.base.Timestamp) * 1000000000
}

// Index implements Block interface.
func (b *bftBlock) Index() uint32 {
	return b.base.Index
}

// NextConsensus implements Block interface.
func (b *bftBlock) NextConsensus() common.Address {
	return b.base.NextConsensus
}

// MerkleRoot implements Block interface.
func (b *bftBlock) MerkleRoot() common.Hash {
	return b.base.MerkleRoot
}

// ConsensusData implements Block interface.
func (b *bftBlock) ConsensusData() uint64 {
	return b.consensusData
}

// Transactions implements Block interface.
func (b *bftBlock) Transactions() []Transaction {
	return b.transactions
}

// SetTransactions implements Block interface.
func (b *bftBlock) SetTransactions(txx []Transaction) {
	b.transactions = txx
}

// NewBlock returns new block.
func NewBlock(timestamp uint64, index uint32, nextConsensus common.Address, prevHash common.Hash, version uint32, nonce uint64, txHashes []common.Hash) Block {
	block := new(bftBlock)
	block.base.Timestamp = uint32(timestamp / 1000000000)
	block.base.Index = index
	block.base.NextConsensus = nextConsensus
	block.base.PrevHash = prevHash
	block.base.Version = version
	block.base.ConsensusData = nonce

	if len(txHashes) != 0 {
		mt := merkle.NewMerkleTree(txHashes...)
		block.base.MerkleRoot = mt.Root().Hash
	}
	return block
}

// Signature implements Block interface.
func (b *bftBlock) Signature() []byte {
	return b.signature
}

// GetHashData returns data for hashing and signing.
// It must be an injection of the set of blocks to the set
// of byte slices, i.e:
// 1. It must have only one valid result for one block.
// 2. Two different blocks must have different hash data.
func (b *bftBlock) GetHashData() []byte {
	w := io.NewBufBinWriter()
	b.EncodeBinary(w.BinWriter)

	return w.Bytes()
}

// Sign implements Block interface.
func (b *bftBlock) Sign(key *keys.PrivateKey) error {
	data := b.GetHashData()

	sign := key.Sign(data)

	b.signature = sign

	return nil
}

// Verify implements Block interface.
func (b *bftBlock) Verify(pub *keys.PublicKey, sign []byte) error {
	data := b.GetHashData()
	if !pub.Verify(sign, data) {
		return errors.New("invalid signature")
	}
	return nil
}

// Hash implements Block interface.
func (b *bftBlock) Hash() (h common.Hash) {
	if b.hash != nil {
		return *b.hash
	} else if b.transactions == nil {
		return
	}

	hash := hash.Keccak256(b.GetHashData())
	b.hash = &hash

	return hash
}

// EncodeBinary implements io.Serializable interface.
func (b base) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(b.Version)
	w.WriteBytes(b.PrevHash[:])
	w.WriteBytes(b.MerkleRoot[:])
	w.WriteU32LE(b.Timestamp)
	w.WriteU32LE(b.Index)
	w.WriteU64LE(b.ConsensusData)
	w.WriteBytes(b.NextConsensus[:])
}

// DecodeBinary implements io.Serializable interface.
func (b *base) DecodeBinary(r *io.BinReader) {
	b.Version = r.ReadU32LE()
	r.ReadBytes(b.PrevHash[:])
	r.ReadBytes(b.MerkleRoot[:])
	b.Timestamp = r.ReadU32LE()
	b.Index = r.ReadU32LE()
	b.ConsensusData = r.ReadU64LE()
	r.ReadBytes(b.NextConsensus[:])
}
