package contract

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/DigitalLabs-web3/neo-go-evm/cli/input"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/options"
	"github.com/DigitalLabs-web3/neo-go-evm/cli/wallet"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response/result"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/urfave/cli"
)

func NewCommands() []cli.Command {
	return []cli.Command{
		{
			Name:        "contract",
			Description: "contract operations",
			Subcommands: []cli.Command{
				{
					Name:   "call",
					Usage:  "call [contractAddress] [abiFilePath] [method] [inputs...]",
					Action: call,
					Flags: append(options.RPC, []cli.Flag{
						wallet.WalletPathFlag,
						wallet.FromAddrFlag,
					}...),
				},
				{
					Name:   "deploy",
					Usage:  "deploy [byteCodeFilePath] [abiFilePath] [inputs...]",
					Action: deploy,
					Flags: append(options.RPC, []cli.Flag{
						wallet.WalletPathFlag,
						wallet.FromAddrFlag,
					}...),
				},
			},
		},
	}
}

func call(ctx *cli.Context) error {
	if len(ctx.Args()) < 3 {
		return cli.NewExitError("parameters not enough", 1)
	}
	address := common.HexToAddress(ctx.Args()[0])
	if address == (common.Address{}) {
		return cli.NewExitError("invalid contract address", 1)
	}
	abiFile := ctx.Args()[1]
	file, err := os.Open(abiFile)
	if err != nil {
		return err
	}
	defer file.Close()
	contractAbi, err := abi.JSON(file)
	if err != nil {
		return err
	}
	method := ctx.Args()[2]
	var inputs []interface{}
	if len(ctx.Args()) > 3 {
		inputs = make([]interface{}, len(ctx.Args())-3)
		for i := 3; i < len(ctx.Args()); i++ {
			pstr := ctx.Args()[i]
			inputs[i-3], err = parseParam(pstr)
			if err != nil {
				return err
			}
		}
	}
	data, err := contractAbi.Pack(method, inputs...)
	if err != nil {
		return err
	}
	facc, err := wallet.DecideFrom(ctx)
	if err != nil {
		return err
	}
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	code, err := c.Eth_GetCode(address)
	if err != nil {
		return err
	}
	if len(code) == 0 {
		return cli.NewExitError("contract not found", 1)
	}
	gasPrice, err := c.Eth_GasPrice()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	ret, err := c.Eth_Call(&result.TransactionObject{
		From:     facc.Address,
		To:       &address,
		Value:    big.NewInt(0),
		GasPrice: gasPrice,
		Data:     data,
	})
	if err != nil {
		return cli.NewExitError(fmt.Errorf("contract call error: %w", err), 1)
	}
	fmt.Fprintf(ctx.App.Writer, "ret: %s\n", hexutil.Encode(ret))
	err = input.ConfirmTx()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	return wallet.MakeEthTx(ctx, facc, &address, big.NewInt(0), data)
}

func deploy(ctx *cli.Context) error {
	if len(ctx.Args()) < 2 {
		return cli.NewExitError("parameters not enough", 1)
	}
	txt, err := os.ReadFile(ctx.Args()[0])
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't read bytecode: %w", err), 1)
	}
	bin, err := hexutil.Decode(strings.Replace(string(txt), "\n", "", -1))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't parse bytecode: %s, %w", string(txt), err), 1)
	}
	abiFile := ctx.Args()[1]
	file, err := os.Open(abiFile)
	if err != nil {
		return err
	}
	defer file.Close()
	contractAbi, err := abi.JSON(file)
	if err != nil {
		return err
	}
	data := bin
	needParamCount := len(contractAbi.Constructor.Inputs)
	if len(ctx.Args())-2 < needParamCount {
		return cli.NewExitError("constructor params not enough", 1)
	}
	if len(ctx.Args())-2 > needParamCount {
		return cli.NewExitError("too many params", 1)
	}
	if needParamCount > 0 {
		inputs := make([]interface{}, needParamCount)
		for i := 3; i < len(ctx.Args()); i++ {
			pstr := ctx.Args()[i]
			inputs[i-3], err = parseParam(pstr)
			if err != nil {
				return err
			}
		}
		arg, err := contractAbi.Constructor.Inputs.Pack(inputs...)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't pack constructor inputs: %w", err), 1)
		}
		data = append(data, arg...)
	}
	facc, err := wallet.DecideFrom(ctx)
	if err != nil {
		return err
	}
	return wallet.MakeEthTx(ctx, facc, nil, big.NewInt(0), data)
}

func parseParam(pstr string) (interface{}, error) {
	if strings.HasPrefix(pstr, "0x") {
		str := pstr[2:]
		if len(str) == 2*common.AddressLength {
			return common.HexToAddress(pstr), nil
		} else if len(str) == 2*common.HashLength {
			return common.HexToHash(pstr), nil
		}
		b, err := hex.DecodeString(str)
		if err != nil {
			return nil, err
		}
		return big.NewInt(0).SetBytes(b), nil
	}
	val, err := strconv.ParseUint(pstr, 10, 32)
	if err == nil {
		return val, nil
	}
	val, err = strconv.ParseUint(pstr, 16, 32)
	if err == nil {
		return val, nil
	}
	b, err := hex.DecodeString(pstr)
	if err == nil {
		return big.NewInt(0).SetBytes(b), nil
	}
	return nil, errors.New("can't parse parameter")
}
