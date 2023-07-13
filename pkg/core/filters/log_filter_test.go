package filters

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestLogFilterJson(t *testing.T) {
	j := `{
		"topics": ["0x000000000000000000000000a94f5374fce5edbc8e2a8697c15331677e6ebf0b"]
	  }`
	lf := &LogFilter{}
	err := lf.UnmarshalJSON([]byte(j))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(lf.Topics))

	k := `{
		"fromBlock": "0x6d9"
	  }`
	lf1 := &LogFilter{}
	err = lf1.UnmarshalJSON([]byte(k))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(lf1.Topics))
	assert.Equal(t, uint64(1753), lf1.FromBlock)
	assert.Equal(t, uint64(0), lf1.ToBlock)

	f := `{
		"blockHash": "0x6d9a51b80a7d82e4396e6b92bd1aadfeba61e843593865beace6ce01f6c6042f"
	  }`
	lf2 := &LogFilter{}
	err = lf2.UnmarshalJSON([]byte(f))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(lf2.Topics))
	assert.Equal(t, common.HexToHash("0x6d9a51b80a7d82e4396e6b92bd1aadfeba61e843593865beace6ce01f6c6042f"), lf2.Blockhash)
}
