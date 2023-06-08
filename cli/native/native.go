package native

import (
	"fmt"
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/cli/options"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/wallet"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli"
)

func NewCommands() []cli.Command {
	return []cli.Command{{
		Name:  "native",
		Usage: "invoke native contract",
		Subcommands: []cli.Command{
			{
				Name:        "policy",
				Usage:       "manage policy",
				Subcommands: newPolicyCommands(),
			},
		},
	},
	}
}

func getNativeContract(ctx *cli.Context, name string) (*state.NativeContract, error) {
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	var err error
	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	natives, err := c.GetNativeContracts()
	if err != nil {
		cli.NewExitError(fmt.Errorf("could not get native contracts: %w", err), 1)
	}
	for _, n := range natives {
		if n.Name == name {
			return &n, nil
		}
	}
	return nil, cli.NewExitError(fmt.Errorf("can't find native contract: %s", name), 1)
}

func callNative(ctx *cli.Context, name string, method string, params ...interface{}) error {
	n, err := getNativeContract(ctx, name)
	if err != nil {
		return err
	}
	data, err := n.Abi.Pack(method, params...)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't pack parameters: %w", err), 1)
	}
	return makeCommitteeTx(ctx, n.Address, data)
}

func makeCommitteeTx(ctx *cli.Context, to common.Address, data []byte) error {
	wall, err := wallet.ReadWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	committeeAddr, err := c.GetConsensusAddress()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed get committee address: %w", err), 1)
	}
	return wallet.MakeNeoTx(ctx, wall, committeeAddr, to, big.NewInt(0), data)
}
