package config

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/network/metrics"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc"
)

// ApplicationConfiguration config specific to the node.
type ApplicationConfiguration struct {
	Address           string                  `yaml:"Address"`
	AnnouncedNodePort uint16                  `yaml:"AnnouncedPort"`
	AttemptConnPeers  int                     `yaml:"AttemptConnPeers"`
	DBConfiguration   storage.DBConfiguration `yaml:"DBConfiguration"`
	DialTimeout       int64                   `yaml:"DialTimeout"`
	LogPath           string                  `yaml:"LogPath"`
	MaxPeers          int                     `yaml:"MaxPeers"`
	MinPeers          int                     `yaml:"MinPeers"`
	NodePort          uint16                  `yaml:"NodePort"`
	PingInterval      int64                   `yaml:"PingInterval"`
	PingTimeout       int64                   `yaml:"PingTimeout"`
	Pprof             metrics.Config          `yaml:"Pprof"`
	Prometheus        metrics.Config          `yaml:"Prometheus"`
	ProtoTickInterval int64                   `yaml:"ProtoTickInterval"`
	Relay             bool                    `yaml:"Relay"`
	RPC               rpc.Config              `yaml:"RPC"`
	UnlockWallet      Wallet                  `yaml:"UnlockWallet"`
	StateRoot         StateRoot               `yaml:"StateRoot"`
	// ExtensiblePoolSize is the maximum amount of the extensible payloads from a single sender.
	ExtensiblePoolSize int `yaml:"ExtensiblePoolSize"`
}
