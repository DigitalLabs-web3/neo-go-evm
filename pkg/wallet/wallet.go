package wallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli"
)

const (
	walletVersion = "3.0"
)

type Wallet struct {
	// Version of the wallet, used for later upgrades.
	Version string `json:"version"`

	// A list of accounts which describes the details of each account
	// in the wallet.
	Accounts []*Account `json:"accounts"`

	Scrypt keys.ScryptParams `json:"scrypt"`

	// Extra metadata can be used for storing arbitrary data.
	// This field can be empty.
	Extra Extra `json:"extra"`

	// Path where the wallet file is located..
	path string
}

// Extra stores imported token contracts.
type Extra struct {
	// Tokens is a list of imported token contracts.
	Tokens []*Token
}

func NewWallet(location string) (*Wallet, error) {
	info, err := os.Stat(location)
	if err == nil && !info.IsDir() {
		return nil, cli.NewExitError("file already exists", 1)
	}
	file, err := os.Create(location)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return newWallet(file), nil
}

// NewWalletFromFile creates a Wallet from the given wallet file path.
func NewWalletFromFile(path string) (*Wallet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	wall := &Wallet{
		path: file.Name(),
	}
	if err := json.NewDecoder(file).Decode(wall); err != nil {
		return nil, err
	}
	return wall, nil
}

func newWallet(rw io.ReadWriter) *Wallet {
	var path string
	if f, ok := rw.(*os.File); ok {
		path = f.Name()
	}
	return &Wallet{
		Version:  walletVersion,
		Accounts: []*Account{},
		Scrypt:   keys.NEP2ScryptParams(),
		path:     path,
	}
}

// CreateAccount generates a new account for the end user and encrypts
// the private key with the given passphrase.
func (w *Wallet) CreateAccount(name, passphrase string) error {
	acc, err := NewAccount()
	if err != nil {
		return err
	}
	acc.Label = name
	if err := acc.Encrypt(passphrase, w.Scrypt); err != nil {
		return err
	}
	w.AddAccount(acc)
	fmt.Printf("account created, address: %s\n", acc.Address)
	return w.Save()
}

// AddAccount adds an existing Account to the wallet.
func (w *Wallet) AddAccount(acc *Account) {
	w.Accounts = append(w.Accounts, acc)
}

// RemoveAccount removes an Account with the specified addr
// from the wallet.
func (w *Wallet) RemoveAccount(addr string) error {
	for i, acc := range w.Accounts {
		if acc.Address == common.HexToAddress(addr) {
			copy(w.Accounts[i:], w.Accounts[i+1:])
			w.Accounts = w.Accounts[:len(w.Accounts)-1]
			return nil
		}
	}
	return errors.New("account wasn't found")
}

// AddToken adds new token to a wallet.
func (w *Wallet) AddToken(tok *Token) {
	w.Extra.Tokens = append(w.Extra.Tokens, tok)
}

// RemoveToken removes token with the specified hash from the wallet.
func (w *Wallet) RemoveToken(h common.Address) error {
	for i, tok := range w.Extra.Tokens {
		if tok.Hash == h {
			copy(w.Extra.Tokens[i:], w.Extra.Tokens[i+1:])
			w.Extra.Tokens = w.Extra.Tokens[:len(w.Extra.Tokens)-1]
			return nil
		}
	}
	return errors.New("token wasn't found")
}

// Path returns the location of the wallet on the filesystem.
func (w *Wallet) Path() string {
	return w.path
}

// Save saves the wallet data. It's the internal io.ReadWriter
// that is responsible for saving the data. This can
// be a buffer, file, etc..
func (w *Wallet) Save() error {
	data, err := json.Marshal(w)
	if err != nil {
		return err
	}

	return w.writeRaw(data)
}

func (w *Wallet) writeRaw(data []byte) error {
	return os.WriteFile(w.path, data, 0644)
}

// JSON outputs a pretty JSON representation of the wallet.
func (w *Wallet) JSON() ([]byte, error) {
	return json.MarshalIndent(w, " ", "	")
}

// Close closes the internal rw if its an io.ReadCloser.
func (w *Wallet) Close() {
}

// GetAccount returns account corresponding to the provided scripthash.
func (w *Wallet) GetAccount(addr common.Address) *Account {
	for _, acc := range w.Accounts {
		if acc.Address == addr {
			return acc
		}
	}

	return nil
}

// GetChangeAddress returns the default address to send transaction's change to.
func (w *Wallet) GetChangeAddress() common.Address {
	return hash.Hash160(w.Accounts[0].PrivateKey().PublicKey().CreateVerificationScript())
}
