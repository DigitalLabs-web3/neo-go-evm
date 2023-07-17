package config

import (
	"errors"
	"sort"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
)

// ProtocolConfiguration represents the protocol config.
type (
	ProtocolConfiguration struct {
		// GarbageCollectionPeriod sets the number of blocks to wait before
		// starting the next MPT garbage collection cycle when RemoveUntraceableBlocks
		// option is used.
		GarbageCollectionPeriod uint32 `yaml:"GarbageCollectionPeriod"`

		ChainID     uint64 `yaml:"ChainID"`
		MemPoolSize int    `yaml:"MemPoolSize"`

		// InitialGASPerValidator is the amount of GAS generated for a validator in the genesis block.
		InitialGASPerValidator uint64 `yaml:"InitialGASPerValidator"`
		// KeepOnlyLatestState specifies if MPT should only store latest state.
		// If true, DB size will be smaller, but older roots won't be accessible.
		// This value should remain the same for the same database.
		KeepOnlyLatestState bool `yaml:"KeepOnlyLatestState"`
		// RemoveUntraceableBlocks specifies if old data should be removed.
		RemoveUntraceableBlocks bool `yaml:"RemoveUntraceableBlocks"`
		// MaxBlockSize is the maximum block size in bytes.
		MaxBlockSize uint32 `yaml:"MaxBlockSize"`
		// MaxBlockSystemFee is the maximum overall system fee per block.
		MaxBlockGas uint64 `yaml:"MaxBlockGas"`
		// MaxTraceableBlocks is the length of the chain accessible to smart contracts.
		MaxTraceableBlocks uint32 `yaml:"MaxTraceableBlocks"`
		// MaxTransactionsPerBlock is the maximum amount of transactions per block.
		MaxTransactionsPerBlock uint16 `yaml:"MaxTransactionsPerBlock"`
		// SaveStorageBatch enables storage batch saving before every persist.
		SaveStorageBatch     bool     `yaml:"SaveStorageBatch"`
		SecondsPerBlock      int      `yaml:"SecondsPerBlock"`
		SeedList             []string `yaml:"SeedList"`
		StandbyValidators    keys.PublicKeys
		StandbyValidatorsStr []string `yaml:"StandbyValidators"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`

		MainNetwork                          uint32 `yaml:"MainNetwork"`
		MainStandbyStateValidatorsScriptHash string `yaml:"MainStandbyStateValidatorsScriptHash"`
		BridgeContractId                     int32  `yaml:"BridgeContractId"`
	}
)

// Validate checks ProtocolConfiguration for internal consistency and returns
// error if anything inappropriate found. Other methods can rely on protocol
// validity after this.
func (p *ProtocolConfiguration) Validate() error {
	standbyValidators, err := keys.NewPublicKeysFromStrings(p.StandbyValidatorsStr)
	if err != nil {
		return err
	}
	standbyValidators = standbyValidators.Unique()
	sort.Sort(standbyValidators)
	p.StandbyValidators = standbyValidators
	if len(p.StandbyValidators) == 0 {
		return errors.New("StandbyValidators can't be empty")
	}
	return nil
}

// GetNumOfCNs returns the number of validators for the given height.
// It implies valid configuration file.
func (p *ProtocolConfiguration) GetNumOfCNs(height uint32) int {
	return len(p.StandbyValidators)
}
