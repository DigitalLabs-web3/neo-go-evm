package block

import (
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Header holds the base info of a block.
type Header struct {
	// Version of the block.
	Version uint32

	// hash of the previous block.
	PrevHash common.Hash

	// Root hash of a transaction list.
	MerkleRoot common.Hash

	// Timestamp is a unix timestamp.
	// The time stamp of each block must be later than previous block's time stamp.
	// Generally the difference of two block's time stamp is about 15 seconds and imprecision is allowed.
	// The height of the block must be exactly equal to the height of the previous block plus 1.
	Timestamp uint64

	// Nonce is block random number.
	Nonce uint64

	// index/height of the block
	Index uint32

	// Contract address of the next miner
	NextConsensus common.Address

	// Script used to validate the block
	Witness transaction.Witness

	// PrimaryIndex is the index of primary consensus node for this block.
	PrimaryIndex byte

	// Hash of this block, created when binary encoded (double SHA256).
	hash common.Hash
}

// Hash returns the hash of the block.
func (b *Header) Hash() common.Hash {
	if b.hash == (common.Hash{}) {
		b.createHash()
	}
	return b.hash
}

// DecodeBinary implements Serializable interface.
func (b *Header) DecodeBinary(br *io.BinReader) {
	b.decodeHashableFields(br)
	b.Witness.DecodeBinary(br)
}

// EncodeBinary implements Serializable interface.
func (b *Header) EncodeBinary(bw *io.BinWriter) {
	b.encodeHashableFields(bw)
	b.Witness.EncodeBinary(bw)
}

// createHash creates the hash of the block.
// When calculating the hash value of the block, instead of calculating the entire block,
// only first seven fields in the block head will be calculated, which are
// version, PrevBlock, MerkleRoot, timestamp, and height, the nonce, NextMiner.
// Since MerkleRoot already contains the hash value of all transactions,
// the modification of transaction will influence the hash value of the block.
func (b *Header) createHash() {
	buf := io.NewBufBinWriter()
	// No error can occur while encoding hashable fields.
	b.encodeHashableFields(buf.BinWriter)

	b.hash = hash.Sha256(buf.Bytes())
}

// encodeHashableFields will only encode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *Header) encodeHashableFields(bw *io.BinWriter) {
	bw.WriteU32LE(b.Version)
	bw.WriteBytes(b.PrevHash[:])
	bw.WriteBytes(b.MerkleRoot[:])
	bw.WriteU64LE(b.Timestamp)
	bw.WriteU64LE(b.Nonce)
	bw.WriteU32LE(b.Index)
	bw.WriteB(b.PrimaryIndex)
	bw.WriteBytes(b.NextConsensus[:])

}

// decodeHashableFields decodes the fields used for hashing.
// see Hash() for more information about the fields.
func (b *Header) decodeHashableFields(br *io.BinReader) {
	b.Version = br.ReadU32LE()
	br.ReadBytes(b.PrevHash[:])
	br.ReadBytes(b.MerkleRoot[:])
	b.Timestamp = br.ReadU64LE()
	b.Nonce = br.ReadU64LE()
	b.Index = br.ReadU32LE()
	b.PrimaryIndex = br.ReadB()
	br.ReadBytes(b.NextConsensus[:])
	// Make the hash of the block here so we dont need to do this
	// again.
	if br.Err == nil {
		b.createHash()
	}
}

type baseJson struct {
	Hash          common.Hash         `json:"hash"`
	Version       hexutil.Uint        `json:"version"`
	PrevHash      common.Hash         `json:"parentHash"`
	MerkleRoot    common.Hash         `json:"transactionsRoot"`
	Timestamp     hexutil.Uint64      `json:"timestamp"`
	Nonce         hexutil.Bytes       `json:"nonce"`
	Index         hexutil.Uint        `json:"number"`
	NextConsensus common.Address      `json:"nextConsensus"`
	PrimaryIndex  hexutil.Uint        `json:"primaryIndex"`
	Witness       transaction.Witness `json:"witness"`
}

// MarshalJSON implements json.Marshaler interface.
func (b Header) MarshalJSON() ([]byte, error) {
	nonceb := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceb, b.Nonce)
	aux := baseJson{
		Hash:          b.Hash(),
		Version:       hexutil.Uint(b.Version),
		PrevHash:      b.PrevHash,
		MerkleRoot:    b.MerkleRoot,
		Timestamp:     hexutil.Uint64(b.Timestamp),
		Nonce:         nonceb,
		Index:         hexutil.Uint(b.Index),
		NextConsensus: b.NextConsensus,
		PrimaryIndex:  hexutil.Uint(b.PrimaryIndex),
		Witness:       b.Witness,
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (b *Header) UnmarshalJSON(data []byte) error {
	var aux = new(baseJson)
	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}
	b.Nonce = binary.BigEndian.Uint64(aux.Nonce)
	b.Version = uint32(aux.Version)
	b.PrevHash = aux.PrevHash
	b.MerkleRoot = aux.MerkleRoot
	b.Timestamp = uint64(aux.Timestamp)
	b.Index = uint32(aux.Index)
	b.NextConsensus = aux.NextConsensus
	b.PrimaryIndex = byte(aux.PrimaryIndex)
	b.Witness = aux.Witness
	if aux.Hash != (b.Hash()) {
		return errors.New("json 'hash' doesn't match block hash")
	}
	return nil
}
