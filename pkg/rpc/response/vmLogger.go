package response

import (
	"math/big"
	"strings"
	"time"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Trace struct {
	parent    *Trace
	CallType  string         `json:"callType"`
	From      common.Address `json:"from"`
	To        common.Address `json:"to"`
	Input     hexutil.Bytes  `json:"input"`
	Gas       hexutil.Uint64 `json:"gas"`
	Value     hexutil.Big    `json:"value"`
	GasUsed   hexutil.Uint64 `json:"gasUsed"`
	Err       string         `json:"error,omitempty"`
	Subtraces []*Trace       `json:"subtraces,omitempty"`
}

type vmLogger struct {
	trace   *Trace
	current *Trace
}

func NewVMLogger() *vmLogger {
	return &vmLogger{}
}

func (vl *vmLogger) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	ip := make([]byte, len(input))
	copy(ip, input)
	vl.trace = &Trace{
		parent:   nil,
		CallType: "call",
		From:     from,
		Input:    hexutil.Bytes(ip),
		Gas:      hexutil.Uint64(gas),
	}
	if value != nil {
		vl.trace.Value = hexutil.Big(*value)
	}
	vl.current = vl.trace
}

func (vl *vmLogger) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {

}

func (vl *vmLogger) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	ip := make([]byte, len(input))
	copy(ip, input)
	t := &Trace{
		parent:   vl.current,
		CallType: strings.ToLower(typ.String()),
		From:     from,
		To:       to,
		Input:    hexutil.Bytes(ip),
		Gas:      hexutil.Uint64(gas),
	}
	if value != nil {
		t.Value = hexutil.Big(*value)
	}
	vl.current.Subtraces = append(vl.current.Subtraces, t)
	vl.current = t
}

func (vl *vmLogger) CaptureExit(output []byte, gasUsed uint64, err error) {
	vl.current.GasUsed = hexutil.Uint64(gasUsed)
	if err != nil {
		vl.current.Err = err.Error()
	}
	vl.current = vl.current.parent
}

func (vl *vmLogger) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error) {
	vl.current.Err = err.Error()
}

func (vl *vmLogger) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
}

func (vl *vmLogger) Result() *Trace {
	return vl.trace
}
