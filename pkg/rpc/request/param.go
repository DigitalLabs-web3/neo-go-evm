package request

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/ethereum/go-ethereum/common"
)

type (
	// Param represents a param either passed to
	// the server or to send to a server using
	// the client.
	Param struct {
		json.RawMessage
		cache interface{}
	}

	// BlockFilter is a wrapper structure for block event filter. The only
	// allowed filter is primary index.
	BlockFilter struct {
		Primary int `json:"primary"`
	}
	// TxFilter is a wrapper structure for transaction event filter. It
	// allows to filter transactions by senders and signers.
	TxFilter struct {
		Sender *common.Address `json:"sender,omitempty"`
	}

	// NotificationFilter is a wrapper structure representing filter used for
	// notifications generated during transaction execution. Notifications can
	// be filtered by contract hash and by name.
	NotificationFilter struct {
		Contract *common.Address `json:"contract,omitempty"`
	}
	// ExecutionFilter is a wrapper structure used for transaction execution
	// events. It allows to choose failing or successful transactions based
	// on their VM state.
	ExecutionFilter struct {
		State uint64 `json:"state"`
	}
	// SignerWithWitness represents transaction's signer with the corresponding witness.
	SignerWithWitness struct {
		Signer common.Address
		transaction.Witness
	}
)

var (
	jsonNullBytes       = []byte("null")
	jsonFalseBytes      = []byte("false")
	jsonTrueBytes       = []byte("true")
	errMissingParameter = errors.New("parameter is missing")
	errNotAString       = errors.New("not a string")
	errNotAnInt         = errors.New("not an integer")
	errNotABool         = errors.New("not a boolean")
	errNotAnArray       = errors.New("not an array")
)

func (p Param) String() string {
	return string(p.RawMessage)
}

// GetStringStrict returns string value of the parameter.
func (p *Param) GetStringStrict() (string, error) {
	if p == nil {
		return "", errMissingParameter
	}
	if p.IsNull() {
		return "", errNotAString
	}
	if p.cache == nil {
		var s string
		err := json.Unmarshal(p.RawMessage, &s)
		if err != nil {
			return "", errNotAString
		}
		p.cache = s
	}
	if s, ok := p.cache.(string); ok {
		return s, nil
	}
	return "", errNotAString
}

// GetString returns string value of the parameter or tries to cast parameter to a string value.
func (p *Param) GetString() (string, error) {
	if p == nil {
		return "", errMissingParameter
	}
	if p.IsNull() {
		return "", errNotAString
	}
	if p.cache == nil {
		var s string
		err := json.Unmarshal(p.RawMessage, &s)
		if err == nil {
			p.cache = s
		} else {
			var i int64
			err = json.Unmarshal(p.RawMessage, &i)
			if err == nil {
				p.cache = i
			} else {
				var b bool
				err = json.Unmarshal(p.RawMessage, &b)
				if err == nil {
					p.cache = b
				} else {
					return "", errNotAString
				}
			}
		}
	}
	switch t := p.cache.(type) {
	case string:
		return t, nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	default:
		return "", errNotAString
	}
}

// GetBooleanStrict returns boolean value of the parameter.
func (p *Param) GetBooleanStrict() (bool, error) {
	if p == nil {
		return false, errMissingParameter
	}
	if bytes.Equal(p.RawMessage, jsonTrueBytes) {
		p.cache = true
		return true, nil
	}
	if bytes.Equal(p.RawMessage, jsonFalseBytes) {
		p.cache = false
		return false, nil
	}
	return false, errNotABool
}

// GetBoolean returns boolean value of the parameter or tries to cast parameter to a bool value.
func (p *Param) GetBoolean() (bool, error) {
	if p == nil {
		return false, errMissingParameter
	}
	if p.IsNull() {
		return false, errNotABool
	}
	var b bool
	if p.cache == nil {
		err := json.Unmarshal(p.RawMessage, &b)
		if err == nil {
			p.cache = b
		} else {
			var s string
			err = json.Unmarshal(p.RawMessage, &s)
			if err == nil {
				p.cache = s
			} else {
				var i int64
				err = json.Unmarshal(p.RawMessage, &i)
				if err == nil {
					p.cache = i
				} else {
					return false, errNotABool
				}
			}
		}
	}
	switch t := p.cache.(type) {
	case bool:
		return t, nil
	case string:
		return t != "", nil
	case int64:
		return t != 0, nil
	default:
		return false, errNotABool
	}
}

// GetIntStrict returns int value of the parameter if the parameter is integer.
func (p *Param) GetIntStrict() (int, error) {
	if p == nil {
		return 0, errMissingParameter
	}
	if p.IsNull() {
		return 0, errNotAnInt
	}
	value, err := p.fillIntCache()
	if err != nil {
		return 0, err
	}
	if i, ok := value.(int64); ok && i == int64(int(i)) {
		return int(i), nil
	}
	return 0, errNotAnInt
}

func (p *Param) fillIntCache() (interface{}, error) {
	if p.cache != nil {
		return p.cache, nil
	}

	// We could also try unmarshalling to uint64, but JSON reliably supports numbers
	// up to 53 bits in size.
	var i int64
	err := json.Unmarshal(p.RawMessage, &i)
	if err == nil {
		p.cache = i
		return i, nil
	}

	var s string
	err = json.Unmarshal(p.RawMessage, &s)
	if err == nil {
		p.cache = s
		return s, nil
	}

	var b bool
	err = json.Unmarshal(p.RawMessage, &b)
	if err == nil {
		p.cache = b
		return b, nil
	}
	return nil, errNotAnInt
}

// GetInt returns int value of the parameter or tries to cast parameter to an int value.
func (p *Param) GetInt() (int, error) {
	if p == nil {
		return 0, errMissingParameter
	}
	if p.IsNull() {
		return 0, errNotAnInt
	}
	value, err := p.fillIntCache()
	if err != nil {
		return 0, err
	}
	switch t := value.(type) {
	case int64:
		if t == int64(int(t)) {
			return int(t), nil
		}
		return 0, errNotAnInt
	case string:
		return strconv.Atoi(t)
	case bool:
		if t {
			return 1, nil
		}
		return 0, nil
	default:
		panic("unreachable")
	}
}

// GetBigInt returns big-interer value of the parameter.
func (p *Param) GetBigInt() (*big.Int, error) {
	if p == nil {
		return nil, errMissingParameter
	}
	if p.IsNull() {
		return nil, errNotAnInt
	}
	value, err := p.fillIntCache()
	if err != nil {
		return nil, err
	}
	switch t := value.(type) {
	case int64:
		return big.NewInt(t), nil
	case string:
		bi, ok := new(big.Int).SetString(t, 10)
		if !ok {
			return nil, errNotAnInt
		}
		return bi, nil
	case bool:
		if t {
			return big.NewInt(1), nil
		}
		return new(big.Int), nil
	default:
		panic("unreachable")
	}
}

// GetArray returns a slice of Params stored in the parameter.
func (p *Param) GetArray() ([]Param, error) {
	if p == nil {
		return nil, errMissingParameter
	}
	if p.IsNull() {
		return nil, errNotAnArray
	}
	if p.cache == nil {
		a := []Param{}
		err := json.Unmarshal(p.RawMessage, &a)
		if err != nil {
			return nil, errNotAnArray
		}
		p.cache = a
	}
	if a, ok := p.cache.([]Param); ok {
		return a, nil
	}
	return nil, errNotAnArray
}

// GetUint256 returns Uint256 value of the parameter.
func (p *Param) GetHash() (common.Hash, error) {
	s, err := p.GetString()
	if err != nil {
		return common.Hash{}, err
	}
	hash := common.HexToHash(s)
	if hash == (common.Hash{}) {
		return common.Hash{}, errors.New("invalid hash")
	}
	return hash, nil
}

// GetUint160FromHex returns Uint160 value of the parameter encoded in hex.
func (p *Param) GetAddressFromHex() (common.Address, error) {
	s, err := p.GetString()
	if err != nil {
		return common.Address{}, err
	}
	if !common.IsHexAddress(s) {
		return common.Address{}, errors.New("invalid address")
	}
	return common.HexToAddress(s), nil
}

// GetBytesHex returns []byte value of the parameter if
// it is a hex-encoded string.
func (p *Param) GetBytesHex() ([]byte, error) {
	s, err := p.GetString()
	if err != nil {
		return nil, err
	}
	s = strings.TrimPrefix(s, "0x")
	return hex.DecodeString(s)
}

func (p *Param) GetBytesBase64() ([]byte, error) {
	s, err := p.GetString()
	if err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(s)
}

// IsNull returns whether parameter represents JSON nil value.
func (p *Param) IsNull() bool {
	return bytes.Equal(p.RawMessage, jsonNullBytes)
}
