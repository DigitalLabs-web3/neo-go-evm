package filters

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type LogFilter struct {
	FromBlock uint64
	ToBlock   uint64
	Blockhash common.Hash
	Address   []common.Address
	Topics    [][]common.Hash
}

func (f *LogFilter) Match(l *types.Log) bool {
	if f.Blockhash != (common.Hash{}) && l.BlockHash != f.Blockhash {
		return false
	}
	if f.FromBlock != 0 && f.ToBlock != 0 {
		if f.Blockhash == (common.Hash{}) && (l.BlockNumber < uint64(f.FromBlock) || l.BlockNumber > uint64(f.ToBlock)) {
			return false
		}
	}

	if len(f.Address) > 0 {
		if !Contains(f.Address, l.Address) {
			return false
		}
	}

	//for _, topic := range f.Topics {
	//	for _, t := range l.Topics {
	//		if topic == t {
	//			return true
	//		}
	//	}
	//}

	return true
}

type logFilterJSON struct {
	FromBlock string           `json:"fromBlock"`
	ToBlock   string           `json:"toBlock"`
	Blockhash common.Hash      `json:"blockHash"`
	Address   []common.Address `json:"address"`
	Topics    [][]common.Hash  `json:"topics"`
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
	input := make(map[string]interface{})
	err1 := json.Unmarshal(b, &input)
	if err1 != nil {
		return err1
	}
	if input["toBlock"] != nil {
		lf.ToBlock = input["toBlock"].(string)
	} else {
		lf.ToBlock = ""
	}
	fmt.Println("input:+++", input)
	if input["fromBlock"] != nil {
		lf.FromBlock = input["fromBlock"].(string)
	} else {
		lf.FromBlock = ""
	}

	if input["blockHash"] != nil {
		lf.Blockhash = common.HexToHash(input["blockHash"].(string))
	} else {
		lf.Blockhash = common.Hash{}
	}

	if input["address"] != nil {
		if address, ok := input["address"].(string); ok {
			lf.Address = []common.Address{common.HexToAddress(address)}
			fmt.Println("address is a string:", address)
		} else if address, ok := input["address"].([]common.Address); ok {
			lf.Address = address
			fmt.Println("address is a []string:", address)
		}
	} else {
		lf.Address = []common.Address{}
	}
	if input["topics"] != nil {
		if topic, ok := input["topics"].([]common.Hash); ok {
			lf.Topics = [][]common.Hash{topic}
			fmt.Println("topic is a []:", topic)
		} else if topic, ok := input["topics"].([][]common.Hash); ok {
			lf.Topics = topic
			fmt.Println("topic is a [][]:", topic)
		}
	} else {
		lf.Topics = [][]common.Hash{}
	}
	//

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
