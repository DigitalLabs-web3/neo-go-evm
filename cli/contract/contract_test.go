package contract

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
)

func TestParseParam(t *testing.T) {
	s := "0x5C651d807c47911B796f97eb07BfaD571d66c738"
	a, err := parseParam(s)
	assert.NoError(t, err)
	_, ok := a.(common.Address)
	assert.True(t, ok)
}

func TestBin(t *testing.T) {
	txt := "0x608060405234801561001057600080fd5b50606460008190555060b6806100276000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c80636d4ce63c14602d575b600080fd5b60336047565b604051603e9190605d565b60405180910390f35b60008054905090565b6057816076565b82525050565b6000602082019050607060008301846050565b92915050565b600081905091905056fea2646970667358221220215af5158d3f8f8fb028514dda1be016c9b40df475faf017125306308975ffdd64736f6c63430008070033"
	_, err := hexutil.Decode(txt)
	assert.NoError(t, err)
}