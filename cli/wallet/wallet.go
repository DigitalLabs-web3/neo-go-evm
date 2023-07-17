package wallet

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/DigitalLabs-web3/neo-go-evm/cli/flags"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/input"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/options"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response/result"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/wallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/urfave/cli"
)

var (
	errNoPath         = errors.New("wallet path is mandatory and should be passed using (--wallet, -w) flags")
	errPhraseMismatch = errors.New("the entered pass-phrases do not match. Maybe you have misspelled them")
	errNoStdin        = errors.New("can't read wallet from stdin for this command")
)

var (
	WalletPathFlag = cli.StringFlag{
		Name:  "wallet, w",
		Usage: "Target location of the wallet file ('-' to read from stdin).",
	}
	keyFlag = cli.StringFlag{
		Name:  "key",
		Usage: "private key to import",
	}
	pswFlag = cli.StringFlag{
		Name:  "psw",
		Usage: "password to encypt private key",
	}
	decryptFlag = flags.AddressFlag{
		Name:  "decrypt, d",
		Usage: "Decrypt encrypted keys.",
	}
	outFlag = cli.StringFlag{
		Name:  "out",
		Usage: "file to put JSON transaction to",
	}
	inFlag = cli.StringFlag{
		Name:  "in",
		Usage: "file with JSON transaction",
	}
	FromAddrFlag = flags.AddressFlag{
		Name:  "from",
		Usage: "Address to send an asset from",
	}
	toAddrFlag = flags.AddressFlag{
		Name:  "to",
		Usage: "Address to send an asset to",
	}
	forceFlag = cli.BoolFlag{
		Name:  "force",
		Usage: "Do not ask for a confirmation",
	}
)

// NewCommands returns 'wallet' command.
func NewCommands() []cli.Command {
	listFlags := []cli.Flag{
		WalletPathFlag,
	}
	listFlags = append(listFlags, options.RPC...)
	return []cli.Command{{
		Name:  "wallet",
		Usage: "create, open and manage a neo-go-evm wallet",
		Subcommands: []cli.Command{
			{
				Name:   "init",
				Usage:  "create a new wallet",
				Action: createWallet,
				Flags: []cli.Flag{
					WalletPathFlag,
					cli.BoolFlag{
						Name:  "account, a",
						Usage: "Create a new account",
					},
				},
			},
			{
				Name:   "change-password",
				Usage:  "change password for accounts",
				Action: changePassword,
				Flags: []cli.Flag{
					WalletPathFlag,
					flags.AddressFlag{
						Name:  "address, a",
						Usage: "address to change password for",
					},
				},
			},
			{
				Name:   "create",
				Usage:  "add an account to the existing wallet",
				Action: addAccount,
				Flags: []cli.Flag{
					WalletPathFlag,
				},
			},
			{
				Name:   "dump-keys",
				Usage:  "dump public keys for account",
				Action: dumpKeys,
				Flags: []cli.Flag{
					WalletPathFlag,
					flags.AddressFlag{
						Name:  "address, a",
						Usage: "address to print public keys for",
					},
				},
			},
			{
				Name:      "export",
				Usage:     "export keys for address",
				UsageText: "export --wallet <path> --decrypt <address>",
				Action:    exportKeys,
				Flags: []cli.Flag{
					WalletPathFlag,
					decryptFlag,
				},
			},
			{
				Name:      "import",
				Usage:     "import private key",
				UsageText: "import --wallet <path> --key <privateKey> --psw <password> [--name <account_name>]",
				Action:    importWallet,
				Flags: []cli.Flag{
					WalletPathFlag,
					keyFlag,
					pswFlag,
					cli.StringFlag{
						Name:  "name, n",
						Usage: "Optional account name",
					},
				},
			},
			{
				Name:  "import-multisig",
				Usage: "import multisig account",
				UsageText: "import-multisig --wallet <path> [--name <account_name>] --min <n>" +
					" [<pubkey1> [<pubkey2> [...]]]",
				Action: importMultisig,
				Flags: []cli.Flag{
					WalletPathFlag,
					cli.StringFlag{
						Name:  "name, n",
						Usage: "Optional account name",
					},
					cli.IntFlag{
						Name:  "min, m",
						Usage: "Minimal number of signatures",
					},
				},
			},
			{
				Name:      "remove",
				Usage:     "remove an account from the wallet",
				UsageText: "remove --wallet <path> [--force] --address <addr>",
				Action:    removeAccount,
				Flags: []cli.Flag{
					WalletPathFlag,
					forceFlag,
					flags.AddressFlag{
						Name:  "address, a",
						Usage: "Account address or hash in LE form to be removed",
					},
				},
			},
			{
				Name:      "list",
				Usage:     "list addresses in wallet",
				UsageText: "list --wallet <path> --rpc-endpoint <node> [--timeout <time>]",
				Action:    listAddresses,
				Flags:     listFlags,
			},
			{
				Name:        "gas",
				Usage:       "work with native gas",
				Subcommands: newNativeTokenCommands(),
			},
		},
	}}
}

func listAddresses(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	for _, acc := range wall.Accounts {
		bal, err := c.Eth_GetBalance(acc.Address)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("could not get balance of account %s, err: %w", acc.Address, err), 1)
		}
		fmt.Fprintf(ctx.App.Writer, "%s GAS: %s\n", acc.Address, bal)
	}
	return nil
}

func changePassword(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		// Check for account presence first before asking for password.
		acc := wall.GetAccount(addrFlag.Address())
		if acc == nil {
			return cli.NewExitError("account is missing", 1)
		}
	}

	oldPass, err := input.ReadPassword("Enter password > ")
	if err != nil {
		return cli.NewExitError(fmt.Errorf("error reading old password: %w", err), 1)
	}

	for i := range wall.Accounts {
		if addrFlag.IsSet && wall.Accounts[i].Address != addrFlag.Address() {
			continue
		}
		err := wall.Accounts[i].Decrypt(oldPass, wall.Scrypt)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("unable to decrypt account %s: %w", wall.Accounts[i].Address, err), 1)
		}
	}

	pass, err := readNewPassword()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("error reading new password: %w", err), 1)
	}
	for i := range wall.Accounts {
		if addrFlag.IsSet && wall.Accounts[i].Address != addrFlag.Address() {
			continue
		}
		err := wall.Accounts[i].Encrypt(pass, wall.Scrypt)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	err = wall.Save()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("error saving the wallet: %w", err), 1)
	}
	return nil
}

func addAccount(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	defer wall.Close()

	if err := createAccount(wall); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func exportKeys(ctx *cli.Context) error {
	wall, err := ReadWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	var addr common.Address

	decrypt := ctx.Generic("decrypt").(*flags.Address)
	if !decrypt.IsSet {
		return cli.NewExitError(fmt.Errorf("missing address to decrypt"), 1)
	}
	addr = decrypt.Address()

	var wifs []string

loop:
	for _, a := range wall.Accounts {
		if a.Address != addr {
			continue
		}
		for i := range wifs {
			if a.EncryptedWIF == wifs[i] {
				continue loop
			}
		}

		wifs = append(wifs, a.EncryptedWIF)
	}
	if len(wifs) == 0 {
		return cli.NewExitError(fmt.Errorf("address not found"), 1)
	}
	for _, wif := range wifs {
		pass, err := input.ReadPassword("Enter password > ")
		if err != nil {
			return cli.NewExitError(fmt.Errorf("error reading password: %w", err), 1)
		}

		pk, err := keys.NEP2Decrypt(wif, pass, wall.Scrypt)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		fmt.Fprintln(ctx.App.Writer, hexutil.Encode(pk.Bytes()))
	}

	return nil
}

func importWallet(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()
	b, err := hexutil.Decode(ctx.String("key"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	key, err := keys.NewPrivateKeyFromBytes(b)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	acc := wallet.NewAccountFromPrivateKey(key)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	pass := ctx.String("psw")
	if err := acc.Encrypt(pass, wall.Scrypt); err != nil {
		return err
	}
	if acc.Label == "" {
		acc.Label = ctx.String("name")
	}
	if err := addAccountAndSave(wall, acc); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func importMultisig(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	m := ctx.Int("min")
	if ctx.NArg() < m {
		return cli.NewExitError(errors.New("insufficient number of public keys"), 1)
	}

	args := []string(ctx.Args())
	pubs := make(keys.PublicKeys, len(args))

	for i := range args {
		pubs[i], err = keys.NewPublicKeyFromString(args[i])
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't decode public key %d: %w", i, err), 1)
		}
	}
	script, err := pubs.CreateMultiSigVerificationScript(m)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't create multisig verification script: %w", err), 1)
	}
	address := hash.Hash160(script)
	acc := &wallet.Account{
		Script:  script,
		Address: address,
	}
	if acc.Label == "" {
		acc.Label = ctx.String("name")
	}
	if err := addAccountAndSave(wall, acc); err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Fprintf(ctx.App.Writer, "Multisig. Addr.: %s \n", address)
	return nil
}

func removeAccount(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	addr := ctx.Generic("address").(*flags.Address)
	if !addr.IsSet {
		return cli.NewExitError("valid account address must be provided", 1)
	}
	acc := wall.GetAccount(addr.Address())
	if acc == nil {
		return cli.NewExitError("account wasn't found", 1)
	}

	if !ctx.Bool("force") {
		fmt.Fprintf(ctx.App.Writer, "Account %s will be removed. This action is irreversible.\n", addr.Address())
		if ok := askForConsent(ctx.App.Writer); !ok {
			return nil
		}
	}

	if err := wall.RemoveAccount(acc.Address.String()); err != nil {
		return cli.NewExitError(fmt.Errorf("error on remove: %w", err), 1)
	}
	if err := wall.Save(); err != nil {
		return cli.NewExitError(fmt.Errorf("error while saving wallet: %w", err), 1)
	}
	return nil
}

func askForConsent(w io.Writer) bool {
	response, err := input.ReadLine("Are you sure? [y/N]: ")
	if err == nil {
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true
		}
	}
	fmt.Fprintln(w, "Cancelled.")
	return false
}

func dumpKeys(ctx *cli.Context) error {
	wall, err := ReadWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	accounts := wall.Accounts

	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		acc := wall.GetAccount(addrFlag.Address())
		if acc == nil {
			return cli.NewExitError("can't find address", 1)
		}
		accounts = []*wallet.Account{acc}
	}

	hasPrinted := false
	for _, acc := range accounts {
		if hasPrinted {
			fmt.Fprintln(ctx.App.Writer)
		}

		fmt.Println("simple signature contract:")
		fmt.Fprintf(ctx.App.Writer, "address: %s \n", acc.Address)
		fmt.Fprintf(ctx.App.Writer, "public key: %s \n", hex.EncodeToString((acc.Script)[1:]))
		hasPrinted = true
	}
	return nil
}

func createWallet(ctx *cli.Context) error {
	path := ctx.String("wallet")
	if len(path) == 0 {
		return cli.NewExitError(errNoPath, 1)
	}
	wall, err := wallet.NewWallet(path)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if err := wall.Save(); err != nil {
		return cli.NewExitError(err, 1)
	}

	if ctx.Bool("account") {
		if err := createAccount(wall); err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	fmtPrintWallet(ctx.App.Writer, wall)
	fmt.Fprintf(ctx.App.Writer, "wallet successfully created, file location is %s\n", wall.Path())
	return nil
}

func readAccountInfo() (string, string, error) {
	name, err := input.ReadLine("Enter the name of the account > ")
	if err != nil {
		return "", "", err
	}
	phrase, err := readNewPassword()
	if err != nil {
		return "", "", err
	}
	return name, phrase, nil
}

func readNewPassword() (string, error) {
	phrase, err := input.ReadPassword("Enter passphrase > ")
	if err != nil {
		return "", fmt.Errorf("error reading password: %w", err)
	}
	phraseCheck, err := input.ReadPassword("Confirm passphrase > ")
	if err != nil {
		return "", fmt.Errorf("error reading password: %w", err)
	}

	if phrase != phraseCheck {
		return "", errPhraseMismatch
	}
	return phrase, nil
}

func createAccount(wall *wallet.Wallet) error {
	name, phrase, err := readAccountInfo()
	if err != nil {
		return err
	}
	return wall.CreateAccount(name, phrase)
}

func openWallet(path string) (*wallet.Wallet, error) {
	if len(path) == 0 {
		return nil, errNoPath
	}
	if path == "-" {
		return nil, errNoStdin
	}
	return wallet.NewWalletFromFile(path)
}

func ReadWallet(path string) (*wallet.Wallet, error) {
	if len(path) == 0 {
		return nil, errNoPath
	}
	if path == "-" {
		w := &wallet.Wallet{}
		if err := json.NewDecoder(os.Stdin).Decode(w); err != nil {
			return nil, fmt.Errorf("js %s", err)
		}
		return w, nil
	}
	return wallet.NewWalletFromFile(path)
}

func addAccountAndSave(w *wallet.Wallet, acc *wallet.Account) error {
	for i := range w.Accounts {
		if w.Accounts[i].Address == acc.Address {
			return fmt.Errorf("address '%s' is already in wallet", acc.Address)
		}
	}

	w.AddAccount(acc)
	return w.Save()
}

func fmtPrintWallet(w io.Writer, wall *wallet.Wallet) {
	b, _ := wall.JSON()
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, string(b))
	fmt.Fprintln(w, "")
}

func DecideFrom(ctx *cli.Context) (*wallet.Account, error) {
	wall, err := ReadWallet(ctx.String("wallet"))
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	var facc *wallet.Account
	fromFlag := ctx.Generic("from").(*flags.Address)
	var from common.Address
	if fromFlag.IsSet {
		from = fromFlag.Address()
		if from == (common.Address{}) {
			return nil, cli.NewExitError(fmt.Errorf("invalid from address"), 1)
		}
		for _, acc := range wall.Accounts {
			if acc.Address == from {
				facc = acc
				break
			}
		}
	} else {
		if len(wall.Accounts) == 0 {
			return nil, cli.NewExitError(fmt.Errorf("could not find any account in wallet"), 1)
		}
		facc = wall.Accounts[0]
		for _, acc := range wall.Accounts {
			if acc.Default {
				facc = acc
				break
			}
		}
	}
	pass, err := input.ReadPassword(fmt.Sprintf("Enter %s password > ", facc.Address))
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("error reading password: %w", err), 1)
	}
	err = facc.Decrypt(pass, wall.Scrypt)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("unable to decrypt account: %s", facc.Address), 1)
	}
	return facc, nil
}

func MakeEthTx(ctx *cli.Context, facc *wallet.Account, to *common.Address, value *big.Int, data []byte) error {
	var err error
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	chainId, err := c.Eth_ChainId()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to get chainId: %w", err), 1)
	}
	gasPrice, err := c.Eth_GasPrice()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	nonce, err := c.Eth_GetTransactionCount(facc.Address)
	if err != nil {
		return err
	}
	ltx := &types.LegacyTx{
		Nonce:    nonce,
		To:       to,
		GasPrice: gasPrice,
		Value:    value,
		Data:     data,
	}
	tx := &transaction.Transaction{
		Transaction: *types.NewTx(ltx),
	}
	gas, err := c.Eth_EstimateGas(&result.TransactionObject{
		From:     facc.Address,
		To:       tx.To(),
		GasPrice: tx.GasPrice(),
		Value:    tx.Value(),
		Data:     tx.Data(),
	})
	if err != nil {
		return err
	}
	ltx.Gas = gas
	tx.Transaction = *types.NewTx(ltx)
	err = facc.SignTx(chainId, tx)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't sign tx: %w", err), 1)
	}
	b, err := tx.Transaction.MarshalBinary()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed encode tx to bytes: %w", err), 1)
	}
	hash, err := c.Eth_SendRawTransaction(b)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed relay tx: %w", err), 1)
	}
	fmt.Fprintf(ctx.App.Writer, "TxHash: %s\n", hash)
	return nil
}
