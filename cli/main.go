package main

import (
	"os"

	"github.com/DigitalLabs-web3/neo-go-evm/cli/contract"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/native"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/query"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/server"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/utils"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/wallet"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/urfave/cli"
)

func main() {
	ctl := newApp()

	if err := ctl.Run(os.Args); err != nil {
		panic(err)
	}
}

func newApp() *cli.App {
	ctl := cli.NewApp()
	ctl.Name = "neo-go-evm"
	ctl.Version = config.Version
	ctl.Usage = "Official Go client for neo-go-evm"
	ctl.ErrWriter = os.Stdout

	ctl.Commands = append(ctl.Commands, server.NewCommands()...)
	ctl.Commands = append(ctl.Commands, wallet.NewCommands()...)
	ctl.Commands = append(ctl.Commands, query.NewCommands()...)
	ctl.Commands = append(ctl.Commands, native.NewCommands()...)
	ctl.Commands = append(ctl.Commands, contract.NewCommands()...)
	ctl.Commands = append(ctl.Commands, utils.NewCommands()...)
	return ctl
}
