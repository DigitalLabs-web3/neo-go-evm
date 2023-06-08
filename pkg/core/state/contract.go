package state

import (
	"encoding/json"
	"reflect"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type Contract struct {
	Address  common.Address `json:"address"`
	CodeHash common.Hash    `json:"codeHash"`
	Code     []byte         `json:"code,omitempty"`
}

// NativeContract holds information about native contract.
type NativeContract struct {
	Name string `json:"name"`
	Contract
	UpdateHistory []uint32 `json:"updatehistory,omitempty"`
	Abi           abi.ABI  `json:"abi"`
	ContractCalls map[string]reflect.Value
}

func (c *Contract) EncodeBinary(bw *io.BinWriter) {
	bw.WriteBytes(c.Address[:])
	bw.WriteBytes(c.CodeHash[:])
	bw.WriteVarBytes(c.Code)
}

func (c *Contract) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(c.Address[:])
	br.ReadBytes(c.CodeHash[:])
	c.Code = br.ReadVarBytes()
}

type nativeJson struct {
	Name          string         `json:"name"`
	Address       common.Address `json:"address"`
	CodeHash      common.Hash    `json:"codeHash"`
	Code          []byte         `json:"code,omitempty"`
	UpdateHistory []uint32       `json:"updatehistory,omitempty"`
	Abi           []fieldJson    `json:"abi"`
}

type argJson struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Components []argJson `json:"components,omitempty"`
	Indexed    bool      `json:"indexed,omitempty"`
}

type fieldJson struct {
	Type    string    `json:"type"`
	Name    string    `json:"name"`
	Inputs  []argJson `json:"inputs"`
	Outputs []argJson `json:"outputs,omitempty"`

	// Status indicator which can be: "pure", "view",
	// "nonpayable" or "payable".
	StateMutability string `json:"stateMutability,omitempty"`

	// Event relevant indicator represents the event is
	// declared as anonymous.
	Anonymous bool `json:"anonymous,omitempty"`
}

func abiToFields(nabi abi.ABI) []fieldJson {
	fields := make([]fieldJson, len(nabi.Methods)+len(nabi.Events))
	i := 0
	for _, method := range nabi.Methods {
		fields[i] = fieldJson{
			Type:            "function",
			Name:            method.Name,
			Inputs:          make([]argJson, len(method.Inputs)),
			Outputs:         make([]argJson, len(method.Outputs)),
			StateMutability: method.StateMutability,
		}
		for j, input := range method.Inputs {
			fields[i].Inputs[j] = argJson{
				Name: input.Name,
				Type: input.Type.String(),
			}
		}
		for j, output := range method.Outputs {
			fields[i].Outputs[j] = argJson{
				Name: output.Name,
				Type: output.Type.String(),
			}
		}
		i++
	}
	for _, event := range nabi.Events {
		fields[i] = fieldJson{
			Type:      "event",
			Name:      event.Name,
			Inputs:    make([]argJson, len(event.Inputs)),
			Anonymous: false,
		}
		for j, input := range event.Inputs {
			fields[i].Inputs[j] = argJson{
				Name: input.Name,
				Type: input.Type.String(),
			}
		}
		i++
	}
	return fields
}

func (n *NativeContract) MarshalJSON() ([]byte, error) {
	return json.Marshal(nativeJson{
		Name:          n.Name,
		Address:       n.Address,
		CodeHash:      n.CodeHash,
		Code:          n.Code,
		UpdateHistory: n.UpdateHistory,
		Abi:           abiToFields(n.Abi),
	})
}
