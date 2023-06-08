package crypto

import "github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"

// VerifiableDecodable represents an object which can be verified and
// those hashable part can be encoded/decoded.
type VerifiableDecodable interface {
	hash.Hashable
	EncodeHashableFields() ([]byte, error)
	DecodeHashableFields([]byte) error
}
