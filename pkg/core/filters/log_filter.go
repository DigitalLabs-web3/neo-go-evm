package filters

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type LogFilter struct {
	FromBlock uint64
	ToBlock   uint64
	Blockhash common.Hash
	Address   []common.Address
	Topics    []common.Hash
}

func (f *LogFilter) Match(l *types.Log) bool {
	if f.Blockhash != (common.Hash{}) && l.BlockHash != f.Blockhash {
		println("hash")
		return false
	}
	if f.FromBlock != 0 && f.ToBlock != 0 {
		if f.Blockhash == (common.Hash{}) && (l.BlockNumber <= uint64(f.FromBlock) || l.BlockNumber >= uint64(f.ToBlock)) {
			println("hash")
			return false
		}
	}

	if len(f.Address) > 0 {
		if !Contains(f.Address, l.Address) {
			println("hash")
			return false
		}
	}

	for _, topic := range f.Topics {
		for _, t := range l.Topics {
			if topic == t {
				return true
			}
		}
	}

	return true
}

type logFilterJSON struct {
	FromBlock string           `json:"fromBlock"`
	ToBlock   string           `json:"toBlock"`
	Blockhash common.Hash      `json:"blockHash"`
	Address   []common.Address `json:"address"`
	Topics    []common.Hash    `json:"topics"`
}

func (f LogFilter) MarshalJSON() ([]byte, error) {
	lf := logFilterJSON{
		FromBlock: hexutil.EncodeUint64(uint64(f.FromBlock)),
		ToBlock:   hexutil.EncodeUint64(uint64(f.ToBlock)),
		Blockhash: f.Blockhash,
		Address:   f.Address,
		Topics:    f.Topics,
	}
	return json.Marshal(lf)
}

func (f *LogFilter) UnmarshalJSON(b []byte) error {
	lf := &logFilterJSON{}
	err := json.Unmarshal(b, lf)
	if err != nil {
		return err
	}
	if lf.FromBlock != "" {
		fromBlock, err := hexutil.DecodeUint64(lf.FromBlock)
		if err != nil {
			return err
		}
		f.FromBlock = fromBlock
	}
	if lf.ToBlock != "" {
		toBlock, err := hexutil.DecodeUint64(lf.ToBlock)
		if err != nil {
			return err
		}
		f.ToBlock = toBlock
	}
	f.Blockhash = lf.Blockhash
	f.Address = lf.Address
	f.Topics = lf.Topics
	return nil
}

func Contains[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}
