package transaction

import (
	"encoding/json"
	"errors"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	// MaxInvocationScript is the maximum length of allowed invocation
	// script. It should fit 11/21 multisignature for the committee.
	MaxInvocationScript = 1024

	// MaxVerificationScript is the maximum allowed length of verification
	// script. It should be appropriate for 11/21 multisignature committee.
	MaxVerificationScript = 1024
)

var (
	ErrInvalidSignature      = errors.New("invalid signature")
	ErrInvalidSignatureCount = errors.New("invalid signatures count")
)

// Witness contains 2 scripts.
type Witness struct {
	InvocationScript   []byte
	VerificationScript []byte
}

// DecodeBinary implements Serializable interface.
func (w *Witness) DecodeBinary(br *io.BinReader) {
	w.InvocationScript = br.ReadVarBytes(MaxInvocationScript)
	w.VerificationScript = br.ReadVarBytes(MaxVerificationScript)
}

// EncodeBinary implements Serializable interface.
func (w *Witness) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarBytes(w.InvocationScript)
	bw.WriteVarBytes(w.VerificationScript)
}

// ScriptHash returns the hash of the VerificationScript.
func (w Witness) Address() common.Address {
	if len(w.VerificationScript) < 1 {
		return common.Address{}
	}
	//single sig
	if w.VerificationScript[0] == 0 {
		pk, err := keys.NewPublicKeyFromBytes(w.VerificationScript[1:], btcec.S256())
		if err != nil {
			return common.Address{}
		}
		return pk.Address()
	} else {
		return hash.Hash160(w.VerificationScript)
	}
}

func (w *Witness) VerifyHashable(chainId uint64, hh hash.Hashable) error {
	if !crypto.IsMultiVerificationScript(w.VerificationScript) {
		pk, err := crypto.ParseVerificationScript(w.VerificationScript)
		if err != nil {
			return err
		}
		r := pk.VerifyHashable(w.InvocationScript, chainId, hh)
		if !r {
			return ErrInvalidSignature
		}
		return nil
	} else {
		pks, m, err := crypto.ParseMultiVerificationScript(w.VerificationScript)
		if err != nil {
			return err
		}
		sigs, err := crypto.ParseMultiInvocationScript(w.InvocationScript)
		if err != nil {
			return err
		}
		if len(sigs) < m {
			return ErrInvalidSignatureCount
		}
		for i, j := 0, 0; i < pks.Len() && j < len(sigs); {
			if pks[i].VerifyHashable(sigs[j], chainId, hh) {
				j++
			}
			i++
			if len(sigs)-j > pks.Len()-i {
				return ErrInvalidSignature
			}
		}
		return nil
	}
}

type witnessJson struct {
	Verification hexutil.Bytes `json:"verification"`
	Invocation   hexutil.Bytes `json:"invocation"`
}

func (w Witness) MarshalJSON() ([]byte, error) {
	wj := witnessJson{
		Verification: w.VerificationScript,
		Invocation:   w.InvocationScript,
	}
	return json.Marshal(wj)
}

func (w *Witness) UnmarshalJSON(b []byte) error {
	wj := new(witnessJson)
	err := json.Unmarshal(b, wj)
	if err != nil {
		return err
	}
	w.VerificationScript = wj.Verification
	w.InvocationScript = wj.Invocation
	return nil
}
