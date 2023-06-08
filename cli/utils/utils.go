package utils

import (
	"fmt"
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/urfave/cli"
)

func NewCommands() []cli.Command {
	return []cli.Command{{
		Name:  "utils",
		Usage: "convert data",
		Subcommands: []cli.Command{
			{
				Name:      "decodeBig",
				Usage:     "convert hex to number",
				Action:    hexToNumber,
				UsageText: "h2n <quantity>",
				ArgsUsage: "<quantity>",
			},
			{
				Name:      "keccak256",
				Usage:     "calculate keccak256 of given hex bytes",
				Action:    keccak256,
				UsageText: "<hexString>",
				ArgsUsage: "<hexString>",
			},
			{
				Name:      "big2Hash",
				Usage:     "convert big number to hash",
				Action:    bigToHash,
				UsageText: "<number: base 10>",
				ArgsUsage: "<number: base 10>",
			},
		},
	}}
}

func hexToNumber(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) < 1 {
		return cli.NewExitError("missing hex string to convert", 1)
	}
	hs := args.First()
	num, err := hexutil.DecodeBig(hs)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	fmt.Fprintf(ctx.App.Writer, "%s\n", num)
	return nil
}

func keccak256(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) < 1 {
		return cli.NewExitError("missing hex string to convert", 1)
	}
	hs := args.First()
	b, err := hexutil.Decode(hs)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	fmt.Fprintln(ctx.App.Writer, hash.Keccak256(b).String())
	return nil
}

func bigToHash(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) < 1 {
		return cli.NewExitError("missing hex string to convert", 1)
	}
	hs := args.First()
	num, ok := big.NewInt(0).SetString(hs, 10)
	if !ok {
		return cli.NewExitError("invalid number: base 10", 1)
	}

	fmt.Fprintln(ctx.App.Writer, common.BigToHash(num).String())
	return nil
}
