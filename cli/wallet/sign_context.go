package wallet

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DigitalLabs-web3/neo-go-evm/cli/input"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/wallet"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type SignContext struct {
	ChainID    uint64
	Tx         transaction.NeoTx
	Parameters map[string][]byte
	M          int
}

func (sc *SignContext) Check() error {
	if sc.Tx.From != hash.Hash160(sc.Tx.Witness.VerificationScript) {
		return errors.New("invalid verification")
	}
	pks, m, err := crypto.ParseMultiVerificationScript(sc.Tx.Witness.VerificationScript)
	if err != nil {
		return err
	}
	if sc.M != m {
		return errors.New("invalid sigs count")
	}
	sc.M = m
	for pkstring, sig := range sc.Parameters {
		if len(sig) > 0 {
			pubkey, err := keys.NewPublicKeyFromString(pkstring)
			if !pks.Contains(pubkey) || err != nil {
				return errors.New("invalid public key")
			}
			if !pubkey.VerifyHashable(sig, sc.ChainID, &sc.Tx) {
				return errors.New("invalid signature")
			}
		}
	}
	return nil
}

func (sc SignContext) IsComplete() bool {
	sigCount := 0
	for _, sig := range sc.Parameters {
		if len(sig) > 0 {
			sigCount++
		}
	}
	return sc.M == sigCount
}

func (sc *SignContext) CreateTx() (*transaction.Transaction, error) {
	if !sc.IsComplete() {
		return nil, errors.New("sign context is incomplete")
	}
	pks, _, err := crypto.ParseMultiVerificationScript(sc.Tx.Witness.VerificationScript)
	if err != nil {
		return nil, errors.New("can't parse multi-sig account script")
	}
	sigs := make([][]byte, sc.M)
	i := 0
	for _, pk := range pks {
		pkstring := hex.EncodeToString(pk.Bytes())
		if sc.Parameters[pkstring] != nil {
			sigs[i] = sc.Parameters[pkstring]
			i++
		}
	}
	sc.Tx.Witness.InvocationScript = crypto.CreateMultiInvocationScript(sigs)
	return transaction.NewTx(&sc.Tx), nil
}

type signContextJson struct {
	ChainID    hexutil.Uint64           `json:"chainId"`
	Tx         transaction.NeoTx        `json:"tx"`
	M          hexutil.Uint64           `json:"m"`
	Parameters map[string]hexutil.Bytes `json:"parameters"`
}

func (sc SignContext) MarshalJSON() ([]byte, error) {
	scj := &signContextJson{
		ChainID: hexutil.Uint64(sc.ChainID),
		Tx:      sc.Tx,
	}
	scj.Parameters = make(map[string]hexutil.Bytes)
	scj.M = hexutil.Uint64(sc.M)
	for k, v := range sc.Parameters {
		scj.Parameters[k] = v
	}
	return json.Marshal(scj)
}

func (sc *SignContext) UnmarshalJSON(b []byte) error {
	scj := new(signContextJson)
	err := json.Unmarshal(b, scj)
	if err != nil {
		return err
	}
	sc.ChainID = uint64(scj.ChainID)
	sc.Tx = scj.Tx
	sc.Parameters = make(map[string][]byte)
	for k, v := range scj.Parameters {
		sc.Parameters[k] = v
	}
	sc.M = int(scj.M)
	err = sc.Check()
	if err != nil {
		return err
	}
	return nil
}

func Sign(wall *wallet.Wallet, context *SignContext) error {
	pks, _, err := crypto.ParseMultiVerificationScript(context.Tx.Witness.VerificationScript)
	if err != nil {
		return fmt.Errorf("can't parse multi-sig account script: %w", err)
	}
	haspk := false
	for _, acc := range wall.Accounts {
		for _, p := range pks {
			if p.Address() == acc.Address {
				haspk = true
				pass, err := input.ReadPassword(fmt.Sprintf("Enter password for %s > ", acc.Address))
				if err != nil {
					return fmt.Errorf("error reading password: %w", err)
				}
				err = acc.Decrypt(pass, wall.Scrypt)
				if err != nil {
					return fmt.Errorf("unable to decrypt account: %s", acc.Address)
				}
				sig := acc.PrivateKey().SignHashable(context.ChainID, &context.Tx)
				context.Parameters[hex.EncodeToString(p.Bytes())] = sig
				if len(context.Parameters) == context.M {
					return nil
				}
			}
		}
	}
	if !haspk {
		return fmt.Errorf("no existent private key in wallet")
	}
	return nil
}
