package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/rfc6979"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/common"
)

type PrivateKey struct {
	ecdsa.PrivateKey
}

// NewPrivateKey creates a new random Secp256k1 private key.
func NewPrivateKey() (*PrivateKey, error) {
	return newPrivateKeyOnCurve(secp256k1.S256())
}

// newPrivateKeyOnCurve creates a new random private key using curve c.
func newPrivateKeyOnCurve(c elliptic.Curve) (*PrivateKey, error) {
	pk, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{*pk}, nil
}

// NewPrivateKeyFromHex returns a Secp256k1 PrivateKey created from the
// given hex string.
func NewPrivateKeyFromHex(str string) (*PrivateKey, error) {
	b, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyFromBytes(b)
}

func NewPrivateKeyFromBytes(b []byte) (*PrivateKey, error) {
	if len(b) != 32 {
		return nil, fmt.Errorf(
			"invalid byte length: expected %d bytes got %d", 32, len(b),
		)
	}
	var (
		c = secp256k1.S256()
		d = new(big.Int).SetBytes(b)
	)

	x, y := c.ScalarBaseMult(b)

	return &PrivateKey{
		ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: c,
				X:     x,
				Y:     y,
			},
			D: d,
		},
	}, nil
}

func NewPrivateKeyFromASN1(b []byte) (*PrivateKey, error) {
	privkey, err := x509.ParseECPrivateKey(b)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{*privkey}, nil
}

// PublicKey derives the uncompressed public key from the private key.
func (p *PrivateKey) PublicKey() *PublicKey {
	result := PublicKey(p.PrivateKey.PublicKey)
	return &result
}

func (p *PrivateKey) Address() common.Address {
	pk := p.PublicKey()
	return pk.Address()
}

// GetScriptHash returns verification script hash for public key associated with
// the private key.
func (p *PrivateKey) GetScriptHash() common.Address {
	pk := p.PublicKey()
	return pk.GetScriptHash()
}

// Sign signs arbitrary length data using the private key. It uses SHA256 to
// calculate hash and then SignHash to create a signature (so you can save on
// hash calculation if you already have it).
func (p *PrivateKey) Sign(data []byte) []byte {
	var digest = sha256.Sum256(data)

	return p.SignHash(digest)
}

// SignHash signs particular hash the private key.
func (p *PrivateKey) SignHash(digest common.Hash) []byte {
	r, s := rfc6979.SignECDSA(&p.PrivateKey, digest[:], sha256.New)
	return getSignatureSlice(p.PrivateKey.Curve, r, s)
}

// SignHashable signs some Hashable item for the network specified using
// hash.NetSha256() with the private key.
func (p *PrivateKey) SignHashable(chainId uint64, hh hash.Hashable) []byte {
	return p.SignHash(hash.NetSha256(chainId, hh))
}

func getSignatureSlice(curve elliptic.Curve, r, s *big.Int) []byte {
	params := curve.Params()
	curveOrderByteSize := params.P.BitLen() / 8
	signature := make([]byte, curveOrderByteSize*2)
	_ = r.FillBytes(signature[:curveOrderByteSize])
	_ = s.FillBytes(signature[curveOrderByteSize:])

	return signature
}

// String implements the stringer interface.
func (p *PrivateKey) String() string {
	return hex.EncodeToString(p.Bytes())
}

// Bytes returns the underlying bytes of the PrivateKey.
func (p *PrivateKey) Bytes() []byte {
	result := make([]byte, 32)
	_ = p.D.FillBytes(result)

	return result
}
