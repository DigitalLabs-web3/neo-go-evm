package hash

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/ripemd160" //nolint:staticcheck // SA1019: package golang.org/x/crypto/ripemd160 is deprecated
	"golang.org/x/crypto/sha3"
)

// Hashable represents an object which can be hashed. Usually these objects
// are io.Serializable and signable. They tend to cache the hash inside for
// effectiveness, providing this accessor method. Anything that can be
// identified with a hash can then be signed and verified.
type Hashable interface {
	Hash() common.Hash
}

func getSignedData(chainId uint64, hh Hashable) []byte {
	var b = make([]byte, 8+32)
	binary.LittleEndian.PutUint64(b, chainId)
	h := hh.Hash()
	copy(b[8:], h[:])
	return b
}

// NetSha256 calculates network-specific hash of Hashable item that can then
// be signed/verified.
func NetSha256(net uint64, hh Hashable) common.Hash {
	return Sha256(getSignedData(net, hh))
}

// Sha256 hashes the incoming byte slice
// using the sha256 algorithm.
func Sha256(data []byte) common.Hash {
	hash := sha256.Sum256(data)
	return hash
}

// DoubleSha256 performs sha256 twice on the given data.
func DoubleSha256(data []byte) common.Hash {
	var hash common.Hash

	h1 := Sha256(data)
	hash = Sha256(h1.Bytes())
	return hash
}

// NetSha256 calculates network-specific hash of Hashable item that can then
// be signed/verified.
func NetKeccak256(chainId uint64, hh Hashable) common.Hash {
	return Keccak256(getSignedData(chainId, hh))
}

// DoubleSha256 performs sha256 twice on the given data.
func DoubleKeccak256(data []byte) common.Hash {
	var hash common.Hash

	h1 := Keccak256(data)
	hash = Keccak256(h1.Bytes())
	return hash
}

// RipeMD160 performs the RIPEMD160 hash algorithm
// on the given data.
func RipeMD160(data []byte) common.Address {
	b := make([]byte, 20)
	hasher := ripemd160.New()
	_, _ = hasher.Write(data)

	hasher.Sum(b[:0])
	return common.BytesToAddress(b)
}

// Hash160 performs sha256 and then ripemd160
// on the given data.
func Hash160(data []byte) common.Address {
	h1 := Sha256(data)
	h2 := RipeMD160(h1.Bytes())
	return h2
}

// Checksum returns the checksum for a given piece of data
// using sha256 twice as the hash algorithm.
func Checksum(data []byte) []byte {
	hash := DoubleKeccak256(data)
	return hash[:4]
}

type KeccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

// NewKeccakState creates a new KeccakState
func NewKeccakState() KeccakState {
	return sha3.NewLegacyKeccak256().(KeccakState)
}

// HashData hashes the provided data using the KeccakState and returns a 32 byte hash
func hashData(kh KeccakState, data []byte) (h common.Hash) {
	kh.Reset()
	kh.Write(data)
	kh.Read(h[:])
	return h
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256(data ...[]byte) (h common.Hash) {
	d := NewKeccakState()
	for _, b := range data {
		d.Write(b)
	}
	d.Read(h[:])
	return h
}

func RlpHash(v interface{}) (h common.Hash) {
	d := NewKeccakState()
	rlp.Encode(d, v)
	d.Read(h[:])
	return h
}
