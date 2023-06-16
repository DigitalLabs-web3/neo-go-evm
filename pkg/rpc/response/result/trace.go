package result

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type TraceResult struct {
	Ret   hexutil.Bytes   `json:"output"`
	Err   string          `json:"error,omitempty"`
	Trace *response.Trace `json:"trace"`
}
