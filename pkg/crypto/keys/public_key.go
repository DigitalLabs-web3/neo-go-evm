package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	lru "github.com/hashicorp/golang-lru"
)

// coordLen is the number of bytes in serialized X or Y coordinate.
const coordLen = 32

// MaxMultiSigCount
const MaxMultiSigCount int = 21

// SignatureLen is the length of standard signature for 256-bit EC key.
const SignatureLen = 64

// Parameters is a list of public keys.
type PublicKeys []*PublicKey

var big0 = big.NewInt(0)
var big3 = big.NewInt(3)

// NewPublicKeysFromStrings converts an array of string-encoded P256 public keys
// into an array of Parameters.
func NewPublicKeysFromStrings(ss []string) (PublicKeys, error) {
	arr := make([]*PublicKey, len(ss))
	for i := range ss {
		pubKey, err := NewPublicKeyFromString(ss[i])
		if err != nil {
			return nil, err
		}
		arr[i] = pubKey
	}
	return PublicKeys(arr), nil
}

func (keys PublicKeys) Len() int      { return len(keys) }
func (keys PublicKeys) Swap(i, j int) { keys[i], keys[j] = keys[j], keys[i] }
func (keys PublicKeys) Less(i, j int) bool {
	return keys[i].Cmp(keys[j]) == -1
}

// DecodeBytes decodes a Parameters from the given slice of bytes.
func (keys *PublicKeys) DecodeBytes(data []byte) error {
	b := io.NewBinReaderFromBuf(data)
	b.ReadArray(keys)
	return b.Err
}

// Bytes encodes Parameters to the new slice of bytes.
func (keys *PublicKeys) Bytes() []byte {
	buf := io.NewBufBinWriter()
	buf.WriteArray(*keys)
	if buf.Err != nil {
		panic(buf.Err)
	}
	return buf.Bytes()
}

// Contains checks whether passed param contained in Parameters.
func (keys PublicKeys) Contains(pKey *PublicKey) bool {
	for _, key := range keys {
		if key.Equal(pKey) {
			return true
		}
	}
	return false
}

// Copy returns copy of keys.
func (keys PublicKeys) Copy() PublicKeys {
	res := make(PublicKeys, len(keys))
	copy(res, keys)
	return res
}

// Unique returns set of public keys.
func (keys PublicKeys) Unique() PublicKeys {
	unique := PublicKeys{}
	for _, publicKey := range keys {
		if !unique.Contains(publicKey) {
			unique = append(unique, publicKey)
		}
	}
	return unique
}

func (keys PublicKeys) CreateMultiSigVerificationScript(m int) ([]byte, error) {
	if m < 1 {
		return nil, fmt.Errorf("param m cannot be smaller or equal to 1 got %d", m)
	}
	if keys.Len() < 1 {
		return nil, fmt.Errorf("could not create multisig for keys cout less equal 1")
	}
	if m > keys.Len() {
		return nil, fmt.Errorf("length of the signatures (%d) is higher then the number of public keys", m)
	}
	if keys.Len() > MaxMultiSigCount {
		return nil, fmt.Errorf("length of the publicKeys (%d) is higher then the limit %d", len(keys), 2*MaxMultiSigCount)
	}
	if m > MaxMultiSigCount {
		return nil, fmt.Errorf("public key count %d exceeds maximum of length 1024", len(keys))
	}
	buf := io.NewBufBinWriter()
	buf.WriteVarUint(uint64(m))
	sort.Sort(keys)
	buf.WriteArray(keys)
	return buf.Bytes(), nil
}

func (keys PublicKeys) CreateDefaultMultiSigRedeemScript() ([]byte, error) {
	n := len(keys)
	m := GetDefaultHonestNodeCount(n)
	return keys.CreateMultiSigVerificationScript(m)
}

func (keys PublicKeys) CreateMajorityMultiSigRedeemScript() ([]byte, error) {
	n := len(keys)
	m := GetMajorityHonestNodeCount(n)
	return keys.CreateMultiSigVerificationScript(m)
}

// PublicKey represents a public key and provides a high level
// API around ecdsa.PublicKey.
type PublicKey ecdsa.PublicKey

// Equal returns true in case public keys are equal.
func (p *PublicKey) Equal(key *PublicKey) bool {
	return p.Curve == key.Curve && p.X.Cmp(key.X) == 0 && p.Y.Cmp(key.Y) == 0
}

// Cmp compares two keys.
func (p *PublicKey) Cmp(key *PublicKey) int {
	xCmp := p.X.Cmp(key.X)
	if xCmp != 0 {
		return xCmp
	}
	return p.Y.Cmp(key.Y)
}

// NewPublicKeyFromString returns a public key created from the
// given hex string.
func NewPublicKeyFromString(s string) (*PublicKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return NewPublicKeyFromBytes(b, secp256k1.S256())
}

// keycache is a simple lru cache for P256 keys that avoids Y calculation overhead
// for known keys.
var keycache *lru.Cache

func init() {
	// Less than 100K, probably enough for our purposes.
	keycache, _ = lru.New(1024)
}

// NewPublicKeyFromBytes returns public key created from b using given EC.
func NewPublicKeyFromBytes(b []byte, curve elliptic.Curve) (*PublicKey, error) {
	var pubKey *PublicKey
	cachedKey, ok := keycache.Get(string(b))
	if ok {
		pubKey = cachedKey.(*PublicKey)
		if pubKey.Curve == curve {
			return pubKey, nil
		}
	}
	pubKey = new(PublicKey)
	pubKey.Curve = curve
	if err := pubKey.DecodeBytes(b); err != nil {
		return nil, err
	}
	keycache.Add(string(b), pubKey)
	return pubKey, nil
}

// getBytes serializes X and Y using compressed or uncompressed format.
func (p *PublicKey) getBytes(compressed bool) []byte {
	if p.IsInfinity() {
		return []byte{0x00}
	}
	if compressed {
		return elliptic.MarshalCompressed(p.Curve, p.X, p.Y)
	}
	return elliptic.Marshal(p.Curve, p.X, p.Y)
}

// Bytes returns byte array representation of the public key in compressed
// form (33 bytes with 0x02 or 0x03 prefix, except infinity which is always 0).
func (p *PublicKey) Bytes() []byte {
	return p.getBytes(true)
}

// UncompressedBytes returns byte array representation of the public key in
// uncompressed form (65 bytes with 0x04 prefix, except infinity which is
// always 0).
func (p *PublicKey) UncompressedBytes() []byte {
	return p.getBytes(false)
}

func NewPublicKeyFromASN1(data []byte) (*PublicKey, error) {
	var (
		err    error
		pubkey interface{}
	)
	if pubkey, err = x509.ParsePKIXPublicKey(data); err != nil {
		return nil, err
	}
	pk, ok := pubkey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("given bytes aren't ECDSA public key")
	}
	result := PublicKey(*pk)
	return &result, nil
}

// decodeCompressedY performs decompression of Y coordinate for given X and Y's least significant bit.
// We use here a short-form Weierstrass curve (https://www.hyperelliptic.org/EFD/g1p/auto-shortw.html)
// y² = x³ + ax + b. Two types of elliptic curves are supported:
// 1. Secp256k1 (Koblitz curve): y² = x³ + b,
// 2. Secp256r1 (Random curve): y² = x³ - 3x + b.
// To decode compressed curve point we perform the following operation: y = sqrt(x³ + ax + b mod p)
// where `p` denotes the order of the underlying curve field.
func decodeCompressedY(x *big.Int, ylsb uint, curve elliptic.Curve) (*big.Int, error) {
	var a *big.Int
	switch curve.(type) {
	case *secp256k1.KoblitzCurve:
		a = big0
	default:
		a = big3
	}
	cp := curve.Params()
	xCubed := new(big.Int).Exp(x, big3, cp.P)
	aX := new(big.Int).Mul(x, a)
	aX.Mod(aX, cp.P)
	ySquared := new(big.Int).Sub(xCubed, aX)
	ySquared.Add(ySquared, cp.B)
	ySquared.Mod(ySquared, cp.P)
	y := new(big.Int).ModSqrt(ySquared, cp.P)
	if y == nil {
		return nil, errors.New("error computing Y for compressed point")
	}
	if y.Bit(0) != ylsb {
		y.Neg(y)
		y.Mod(y, cp.P)
	}
	return y, nil
}

// DecodeBytes decodes a PublicKey from the given slice of bytes.
func (p *PublicKey) DecodeBytes(data []byte) error {
	b := io.NewBinReaderFromBuf(data)
	p.DecodeBinary(b)
	if b.Err != nil {
		return b.Err
	}

	if b.Len() != 0 {
		return errors.New("extra data")
	}
	return nil
}

// DecodeBinary decodes a PublicKey from the given BinReader using information
// about the EC curve to decompress Y point. Secp256r1 is a default value for EC curve.
func (p *PublicKey) DecodeBinary(r *io.BinReader) {
	var prefix uint8
	var x, y *big.Int
	var err error

	prefix = uint8(r.ReadB())
	if r.Err != nil {
		return
	}

	if p.Curve == nil {
		p.Curve = secp256k1.S256()
	}
	curve := p.Curve
	curveParams := p.Params()
	// Infinity
	switch prefix {
	case 0x00:
		// noop, initialized to nil
		return
	case 0x02, 0x03:
		// Compressed public keys
		xbytes := make([]byte, coordLen)
		r.ReadBytes(xbytes)
		if r.Err != nil {
			return
		}
		x = new(big.Int).SetBytes(xbytes)
		ylsb := uint(prefix & 0x1)
		y, err = decodeCompressedY(x, ylsb, curve)
		if err != nil {
			r.Err = err
			return
		}
	case 0x04:
		xbytes := make([]byte, coordLen)
		ybytes := make([]byte, coordLen)
		r.ReadBytes(xbytes)
		r.ReadBytes(ybytes)
		if r.Err != nil {
			return
		}
		x = new(big.Int).SetBytes(xbytes)
		y = new(big.Int).SetBytes(ybytes)
		if !curve.IsOnCurve(x, y) {
			r.Err = errors.New("encoded point is not on the P256 curve")
			return
		}
	default:
		r.Err = fmt.Errorf("invalid prefix %d", prefix)
		return
	}
	if x.Cmp(curveParams.P) >= 0 || y.Cmp(curveParams.P) >= 0 {
		r.Err = errors.New("enccoded point is not correct (X or Y is bigger than P")
		return
	}
	p.X, p.Y = x, y
}

// EncodeBinary encodes a PublicKey to the given BinWriter.
func (p *PublicKey) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(p.Bytes())
}

// GetScriptHash returns a Hash160 of verification script for the key.
func (p PublicKey) GetScriptHash() common.Address {
	return hash.Hash160(p.CreateVerificationScript())
}

func (p PublicKey) CreateVerificationScript() []byte {
	buf := io.NewBufBinWriter()
	buf.WriteVarUint(0)
	p.EncodeBinary(buf.BinWriter)
	return buf.Bytes()
}

func (p PublicKey) Address() common.Address {
	return crypto.PubkeyToAddress(ecdsa.PublicKey(p))
}

// Verify returns true if the signature is valid and corresponds
// to the hash and public key.
func (p *PublicKey) Verify(signature []byte, hash []byte) bool {
	if p.X == nil || p.Y == nil || len(signature) != SignatureLen {
		return false
	}
	rBytes := new(big.Int).SetBytes(signature[0:32])
	sBytes := new(big.Int).SetBytes(signature[32:64])
	return ecdsa.Verify((*ecdsa.PublicKey)(p), hash, rBytes, sBytes)
}

// VerifyHashable returns true if the signature is valid and corresponds
// to the hash and public key.
func (p *PublicKey) VerifyHashable(signature []byte, chainId uint64, hh hash.Hashable) bool {
	var digest = hash.NetSha256(chainId, hh)
	return p.Verify(signature, digest[:])
}

// IsInfinity checks if the key is infinite (null, basically).
func (p *PublicKey) IsInfinity() bool {
	return p.X == nil && p.Y == nil
}

// String implements the Stringer interface.
func (p *PublicKey) String() string {
	if p.IsInfinity() {
		return "00"
	}
	bx := hex.EncodeToString(p.X.Bytes())
	by := hex.EncodeToString(p.Y.Bytes())
	return fmt.Sprintf("%s%s", bx, by)
}

// MarshalJSON implements the json.Marshaler interface.
func (p PublicKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(p.Bytes()))
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (p *PublicKey) UnmarshalJSON(data []byte) error {
	l := len(data)
	if l < 2 || data[0] != '"' || data[l-1] != '"' {
		return errors.New("wrong format")
	}

	bytes := make([]byte, hex.DecodedLen(l-2))
	_, err := hex.Decode(bytes, data[1:l-1])
	if err != nil {
		return err
	}
	err = p.DecodeBytes(bytes)
	if err != nil {
		return err
	}

	return nil
}
