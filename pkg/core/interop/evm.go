package interop

import (
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

type EVM struct {
	*vm.EVM
	ChainConfig *params.ChainConfig
}

func NewEVM(bctx vm.BlockContext,
	tctx vm.TxContext,
	sdb vm.StateDB,
	protocolSettings config.ProtocolConfiguration,
	nativeContracts map[common.Address]vm.NativeContract) *EVM {
	chainConfig := &params.ChainConfig{
		ChainID:             big.NewInt(int64(protocolSettings.ChainID)),
		HomesteadBlock:      big.NewInt(0),
		DAOForkBlock:        nil,
		DAOForkSupport:      true,
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		Ethash:              new(params.EthashConfig),
	}
	evm := vm.NewEVM(bctx, tctx, sdb, chainConfig, vm.Config{}, nativeContracts)
	return &EVM{
		EVM:         evm,
		ChainConfig: chainConfig,
	}
}
