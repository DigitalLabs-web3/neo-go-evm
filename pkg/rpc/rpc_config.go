package rpc

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/wallet"
)

type (
	// Config is an RPC service configuration information.
	Config struct {
		Address              string `yaml:"Address"`
		Enabled              bool   `yaml:"Enabled"`
		EnableCORSWorkaround bool   `yaml:"EnableCORSWorkaround"`
		// MaxGasInvoke is a maximum amount of gas which
		// can be spent during RPC call.
		MaxGasInvoke           uint64    `yaml:"MaxGasInvoke"`
		MaxIteratorResultItems int       `yaml:"MaxIteratorResultItems"`
		MaxFindResultItems     int       `yaml:"MaxFindResultItems"`
		MaxERC721Tokens        int       `yaml:"MaxERC721Tokens"`
		Port                   uint16    `yaml:"Port"`
		TLSConfig              TLSConfig `yaml:"TLSConfig"`
		Wallet                 wallet.Wallet
	}

	// TLSConfig describes SSL/TLS configuration.
	TLSConfig struct {
		Address  string `yaml:"Address"`
		CertFile string `yaml:"CertFile"`
		Enabled  bool   `yaml:"Enabled"`
		Port     uint16 `yaml:"Port"`
		KeyFile  string `yaml:"KeyFile"`
	}
)
