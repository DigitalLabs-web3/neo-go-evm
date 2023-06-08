package crypto

import (
	"errors"
	"sort"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/btcsuite/btcd/btcec/v2"
)

var (
	ErrEmptyScript                    = errors.New("empty script")
	ErrInvalidVerificationScript      = errors.New("invalid single verification script")
	ErrInvalidMultiVerificationScript = errors.New("invalid multiple verification script")
	ErrInvalidInvocationScript        = errors.New("invalid invocation script")
	ErrPublicKeyCountExceedsLimit     = errors.New("public key count exceeds limit")
	ErrMExceedsPublicKeyCount         = errors.New("length of the signatures is higher then the number of public keys")
	ErrInvalidSignaturesCount         = errors.New("invalid signatures count")
)

func CreateMultiInvocationScript(sigs [][]byte) []byte {
	buf := io.NewBufBinWriter()
	buf.WriteVarUint(uint64(len(sigs)))
	for _, sig := range sigs {
		buf.WriteVarBytes(sig)
	}
	return buf.Bytes()
}

func ParseMultiInvocationScript(script []byte) ([][]byte, error) {
	br := io.NewBinReaderFromBuf(script)
	n := br.ReadVarUint()
	if br.Err != nil {
		return nil, br.Err
	}
	sigs := make([][]byte, n)
	for i := uint64(0); i < n; i++ {
		sigs[i] = br.ReadVarBytes()
		if br.Err != nil {
			return nil, br.Err
		}
	}
	return sigs, nil
}

func IsMultiVerificationScript(script []byte) bool {
	return len(script) > 1 && script[0] > 0
}

func ParseMultiVerificationScript(script []byte) (pks keys.PublicKeys, m int, err error) {
	if len(script) < 1 {
		err = ErrEmptyScript
		return
	}
	m = int(script[0])
	if m < 1 {
		err = ErrInvalidMultiVerificationScript
		return
	}
	if m > keys.MaxMultiSigCount {
		err = ErrPublicKeyCountExceedsLimit
		return
	}
	pks = keys.PublicKeys{}
	err = pks.DecodeBytes(script[1:])
	if err != nil {
		return
	}
	tks := pks.Unique()
	pks = tks
	sort.Sort(pks)
	if pks.Len() > keys.MaxMultiSigCount {
		err = ErrPublicKeyCountExceedsLimit
		return
	}
	if m > pks.Len() {
		err = ErrMExceedsPublicKeyCount
	}
	return
}

func ParseVerificationScript(script []byte) (*keys.PublicKey, error) {
	if len(script) < 1 {
		return nil, ErrEmptyScript
	}
	if script[0] != 0 {
		return nil, ErrInvalidVerificationScript
	}
	return keys.NewPublicKeyFromBytes(script[1:], btcec.S256())
}
