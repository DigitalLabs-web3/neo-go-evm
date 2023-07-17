package native

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	defaultNativeReadFee     = 200
	defaultNativeWriteFee    = 5000
	contractMethodNamePrefix = "ContractCall"
)

var (
	ErrEmptyInput                = errors.New("empty input")
	ErrInvalidInput              = errors.New("invalid input")
	ErrInvalidMethodID           = errors.New("invalid method id")
	ErrEmptyNodeList             = errors.New("node list is empty")
	ErrLargeNodeList             = errors.New("node list is too large")
	ErrInvalidRole               = errors.New("invalid role")
	ErrInvalidSender             = errors.New("sender check failed")
	ErrNoBlock                   = errors.New("no persisting block in the context")
	ErrInitialize                = errors.New("initialize should only execute in genesis block")
	ErrInvalidContractCallInputs = errors.New("need at least 1 for InteropContext in contract call")
	ErrInvalidContractCallReturn = errors.New("invalid return value in contract call")
)

type Contracts struct {
	GAS        *GAS
	Ledger     *Ledger
	Designate  *Designate
	Management *Management
	Policy     *Policy
	Bridge     *Bridge
	Contracts  []state.NativeContract
}

func NewContracts(cfg config.ProtocolConfiguration) *Contracts {
	cs := &Contracts{
		Contracts: make([]state.NativeContract, 0, 4),
	}
	cs.GAS = NewGAS(cs, cfg.InitialGASPerValidator)
	cs.Contracts = append(cs.Contracts, cs.GAS.NativeContract)
	cs.Ledger = NewLedger()
	cs.Contracts = append(cs.Contracts, cs.Ledger.NativeContract)
	cs.Management = NewManagement(cs)
	cs.Contracts = append(cs.Contracts, cs.Management.NativeContract)
	cs.Designate = NewDesignate(cfg)
	cs.Contracts = append(cs.Contracts, cs.Designate.NativeContract)
	cs.Policy = NewPolicy(cs)
	cs.Contracts = append(cs.Contracts, cs.Policy.NativeContract)
	cs.Bridge = NewBridge(cs, cfg)
	cs.Contracts = append(cs.Contracts, cs.Bridge.NativeContract)
	return cs
}

func (cs *Contracts) ByName(name string) *state.NativeContract {
	name = strings.ToLower(name)
	for _, ctr := range cs.Contracts {
		if strings.ToLower(ctr.Name) == name {
			return &ctr
		}
	}
	return nil
}

func (cs *Contracts) OnPersist(d *dao.Simple, block *block.Block) error {
	if block.Index == 0 {
		err := cs.Designate.Initialize(d)
		if err != nil {
			return err
		}
		err = cs.Policy.Initialize(d)
		if err != nil {
			return err
		}
		err = cs.GAS.Initialize(d)
		if err != nil {
			return err
		}
		err = cs.Management.Initialize(d)
		if err != nil {
			return err
		}
	}
	return cs.GAS.OnPersist(d, block)
}

func (cs *Contracts) PostPersist(d *dao.Simple, block *block.Block) error {
	err := cs.Designate.PostPersist(d, block)
	if err != nil {
		return err
	}
	return cs.Policy.PostPersist(d, block)
}

func convertType(in reflect.Type) (abi.Type, error) {
	switch in.Kind() {
	case reflect.Array, reflect.Slice:
		switch in.Elem().Kind() {
		case reflect.Uint8:
			if in.Size() == common.AddressLength && in.Name() == "Address" {
				return abi.NewType("address", "address", nil)
			}
			if in.Size() == common.HashLength && in.Name() == "Hash" {
				return abi.NewType("uint256", "uint256", nil)
			}
			return abi.NewType("bytes", "bytes", nil)
		default:
			return abi.Type{}, fmt.Errorf("invalid array element type: %s", in.Elem().Name())
		}
	case reflect.Uint8:
		return abi.NewType("uint8", "uint8", nil)
	case reflect.Uint32:
		return abi.NewType("uint32", "uint32", nil)
	case reflect.Uint64:
		return abi.NewType("uint64", "uint64", nil)
	case reflect.Int64:
		return abi.NewType("int64", "int64", nil)
	case reflect.Ptr:
		if in == reflect.TypeOf(big.NewInt(0)) {
			return abi.NewType("uint256", "uint256", nil)
		}
		return abi.Type{}, fmt.Errorf("invalid pointer type: %s", in)
	default:
		return abi.Type{}, fmt.Errorf("invalid type: %s", in.Name())
	}
}

func parseMethodName(method string) (name string, mutability string, isContractCall bool) {
	ss := strings.Split(method, "_")
	if len(ss) < 2 {
		isContractCall = false
		return
	}
	if ss[0] != contractMethodNamePrefix {
		isContractCall = false
		return
	}
	isContractCall = true
	ss = ss[1:]
	name = ss[len(ss)-1]
	mutability = "nonpayable"
	ss = ss[:(len(ss) - 1)]
	if len(ss) == 0 {
		return
	}
	if ss[0] == "Payble" {
		mutability = "payable"
	}
	ss = ss[1:]
	if len(ss) == 0 {
		return
	}
	if ss[0] == "View" {
		mutability += " view"
	}
	return
}

func constructAbi(any interface{}) (*abi.ABI, map[string]reflect.Value, error) {
	const paramNames = "abcdefghijklmnopqrstuvwxyz"
	a := new(abi.ABI)
	a.Methods = make(map[string]abi.Method)
	a.Events = make(map[string]abi.Event)
	a.Errors = make(map[string]abi.Error)
	contractCalls := make(map[string]reflect.Value)
	ty := reflect.TypeOf(any)
	for i := 0; i < ty.NumMethod(); i++ {
		method := ty.Method(i)
		if name, mutability, isContractCall := parseMethodName(method.Name); isContractCall {
			if len(name) == 0 {
				continue
			}
			//ignore method receiver and interop context
			numIn := method.Type.NumIn() - 2
			if numIn < 0 {
				return nil, contractCalls, ErrInvalidContractCallInputs
			}
			inputs := make([]abi.Argument, numIn)
			for i := 0; i < numIn; i++ {
				in := method.Type.In(i + 2)
				ty, err := convertType(in)
				if err != nil {
					return nil, contractCalls, fmt.Errorf("can't convert %d input of %s: %w", i, name, err)
				}
				inputs[i] = abi.Argument{
					Name: string(paramNames[i]),
					Type: ty,
				}
			}
			numOut := method.Type.NumOut()
			if numOut < 1 || numOut > 2 {
				return nil, contractCalls, ErrInvalidContractCallReturn
			}
			outE := method.Type.Out(numOut - 1)
			if !outE.Implements(reflect.TypeOf(new(error)).Elem()) {
				return nil, contractCalls, ErrInvalidContractCallReturn
			}
			outputs := abi.Arguments{}
			if numOut > 1 {
				b := method.Type.Out(0)
				if (b.Kind() != reflect.Array && b.Kind() != reflect.Slice) || b.Elem().Kind() != reflect.Uint8 {
					return nil, contractCalls, ErrInvalidContractCallReturn
				}
				r, err := abi.NewType("bytes", "bytes", nil)
				if err != nil {
					return nil, contractCalls, err
				}
				outputs = append(outputs, abi.Argument{
					Name: "result",
					Type: r,
				})
			}
			contractCalls[name] = method.Func
			if len(inputs) > 0 {
				a.Events[name] = abi.NewEvent(name, name, false, inputs)
			}
			meth := abi.NewMethod(name, name, abi.Function, mutability, false, strings.Contains(mutability, "payble"), inputs, outputs)
			a.Methods[name] = meth
		}
	}
	return a, contractCalls, nil
}

func contractCall(contract interface{}, nativeContract *state.NativeContract, ic InteropContext, input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, errors.New("invalid input")
	}
	method, err := nativeContract.Abi.MethodById(input[:4])
	if err != nil {
		return nil, err
	}
	if method == nil {
		return nil, errors.New("method not found")
	}
	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	vals := make([]reflect.Value, len(args))
	for i, arg := range args {
		vals[i] = reflect.ValueOf(arg)
	}
	rs := nativeContract.ContractCalls[method.Name].Call(append([]reflect.Value{reflect.ValueOf(contract), reflect.ValueOf(ic)}, vals...))
	if len(rs) < 1 || len(rs) > 2 {
		return nil, ErrInvalidContractCallReturn
	}
	if !rs[len(rs)-1].IsNil() {
		e, ok := rs[len(rs)-1].Interface().(error)
		if !ok {
			return nil, ErrInvalidContractCallReturn
		}
		return nil, e
	}
	if len(rs) == 1 {
		return nil, nil
	}
	r, ok := rs[0].Interface().([]byte)
	if !ok {
		return nil, ErrInvalidContractCallReturn
	}
	return r, nil
}

func log(ic InteropContext, contract common.Address, data []byte, topics ...common.Hash) {
	ic.Log(&types.Log{
		Address:     contract,
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ic.PersistingBlock().Index),
	})
}
