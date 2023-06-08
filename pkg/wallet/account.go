package wallet

import (
	"errors"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type Account struct {
	privateKey *keys.PrivateKey

	Script hexutil.Bytes `json:"script"`

	Address common.Address `json:"address"`

	// Encrypted WIF of the account also known as the key.
	EncryptedWIF string `json:"key"`

	// Label is a label the user had made for this account.
	Label string `json:"label"`

	// Indicates whether the account is locked by the user.
	// the client shouldn't spend the funds in a locked account.
	Locked bool `json:"lock"`

	// Indicates whether the account is the default change account.
	Default bool `json:"isDefault"`
}

// NewAccount creates a new Account with a random generated PrivateKey.
func NewAccount() (*Account, error) {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	return NewAccountFromPrivateKey(priv), nil
}

func (a *Account) IsMultiSig() bool {
	return len(a.Script) > 0 && a.Script[0] > 0
}

// SignTx signs transaction t and updates it's Witnesses.
func (a *Account) SignTx(chainId uint64, t *transaction.Transaction) error {
	if a.privateKey == nil {
		return errors.New("account is not unlocked")
	}
	switch t.Type {
	case transaction.NeoTxType:
		sig := a.privateKey.SignHashable(chainId, t)
		witness := transaction.Witness{
			VerificationScript: (*a.privateKey.PublicKey()).CreateVerificationScript(),
			InvocationScript:   sig,
		}
		t.WithWitness(witness)
		return nil
	case transaction.EthTxType:
		sig, err := crypto.Sign(t.SignHash(chainId).Bytes(), &a.privateKey.PrivateKey)
		if err != nil {
			return err
		}
		t.WithSignature(chainId, sig)
		return nil
	default:
		return transaction.ErrUnsupportType
	}
}

// Decrypt decrypts the EncryptedWIF with the given passphrase returning error
// if anything goes wrong.
func (a *Account) Decrypt(passphrase string, scrypt keys.ScryptParams) error {
	var err error

	if a.EncryptedWIF == "" {
		return errors.New("no encrypted wif in the account")
	}
	a.privateKey, err = keys.NEP2Decrypt(a.EncryptedWIF, passphrase, scrypt)
	if err != nil {
		return err
	}

	a.Script = append([]byte{0}, a.privateKey.PublicKey().Bytes()...)
	return nil
}

// Encrypt encrypts the wallet's PrivateKey with the given passphrase
// under the NEP-2 standard.
func (a *Account) Encrypt(passphrase string, scrypt keys.ScryptParams) error {
	wif, err := keys.NEP2Encrypt(a.privateKey, passphrase, scrypt)
	if err != nil {
		return err
	}
	a.EncryptedWIF = wif
	return nil
}

// PrivateKey returns private key corresponding to the account.
func (a *Account) PrivateKey() *keys.PrivateKey {
	return a.privateKey
}

// NewAccountFromEncryptedWIF creates a new Account from the given encrypted WIF.
func NewAccountFromEncryptedWIF(wif string, pass string, scrypt keys.ScryptParams) (*Account, error) {
	priv, err := keys.NEP2Decrypt(wif, pass, scrypt)
	if err != nil {
		return nil, err
	}

	a := NewAccountFromPrivateKey(priv)
	a.EncryptedWIF = wif
	return a, nil
}

// NewAccountFromPrivateKey creates a wallet from the given PrivateKey.
func NewAccountFromPrivateKey(p *keys.PrivateKey) *Account {
	pubKey := p.PublicKey()

	a := &Account{
		Script:     append([]byte{0}, pubKey.Bytes()...),
		privateKey: p,
		Address:    pubKey.Address(),
	}

	return a
}
