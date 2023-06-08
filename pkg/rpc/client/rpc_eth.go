package client

import (
	"math/big"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/filters"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/request"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response/result"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

func (c *Client) Web3_ClientVersion() (string, error) {
	var (
		params = request.NewRawParams()
		resp   = ""
	)
	if err := c.performRequest("web3_clientVersion", params, &resp); err != nil {
		return "", err
	}
	return resp, nil
}

func (c *Client) Web3_Sha3(data []byte) (common.Hash, error) {
	var (
		params = request.NewRawParams(hexutil.Encode(data))
		resp   = common.Hash{}
	)
	if err := c.performRequest("web3_sha3", params, &resp); err != nil {
		return common.Hash{}, err
	}
	return resp, nil
}

func (c *Client) Net_Version() (string, error) {
	var (
		params = request.NewRawParams()
		resp   = ""
	)
	if err := c.performRequest("net_version", params, &resp); err != nil {
		return "", err
	}
	return resp, nil
}

func (c *Client) Net_PeerCount() (uint64, error) {
	var (
		params = request.NewRawParams()
		resp   = ""
	)
	if err := c.performRequest("net_peerCount", params, &resp); err != nil {
		return 0, err
	}
	return hexutil.DecodeUint64(resp)
}

func (c *Client) Net_Listening() (bool, error) {
	var (
		params = request.NewRawParams()
		resp   = false
	)
	if err := c.performRequest("net_listening", params, &resp); err != nil {
		return false, err
	}
	return resp, nil
}

func (c *Client) Eth_ProtocolVersion() (string, error) {
	var (
		params = request.NewRawParams()
		resp   = ""
	)
	if err := c.performRequest("eth_protocolVersion", params, &resp); err != nil {
		return "", err
	}
	return resp, nil
}

func (c *Client) Eth_ChainId() (uint64, error) {
	var (
		params = request.NewRawParams()
		resp   = ""
	)
	if err := c.performRequest("eth_chainId", params, &resp); err != nil {
		return 0, err
	}
	return hexutil.DecodeUint64(resp)
}

func (c *Client) Eth_Syncing() (*result.Syncing, error) {
	var (
		params = request.NewRawParams()
		resp   = &result.Syncing{}
	)
	if err := c.performRequest("eth_syncing", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Eth_Coinbase() (common.Address, error) {
	var (
		params = request.NewRawParams()
		resp   = common.Address{}
	)
	if err := c.performRequest("eth_coinbase", params, &resp); err != nil {
		return common.Address{}, err
	}
	return resp, nil
}

func (c *Client) Eth_GasPrice() (*big.Int, error) {
	var (
		params = request.NewRawParams()
		resp   = ""
	)
	if err := c.performRequest("eth_gasPrice", params, &resp); err != nil {
		return nil, err
	}
	return hexutil.DecodeBig(resp)
}

func (c *Client) Eth_Accounts() ([]common.Address, error) {
	var (
		params = request.NewRawParams()
		resp   = []common.Address{}
	)
	if err := c.performRequest("eth_accounts", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Eth_BlockNumber() (uint64, error) {
	var (
		params = request.NewRawParams()
		resp   = ""
	)
	if err := c.performRequest("eth_blockNumber", params, &resp); err != nil {
		return 0, err
	}
	return hexutil.DecodeUint64(resp)
}

func (c *Client) Eth_GetBalance(addr common.Address) (*big.Int, error) {
	var (
		params = request.NewRawParams(addr.String())
		resp   = ""
	)
	if err := c.performRequest("eth_getBalance", params, &resp); err != nil {
		return nil, err
	}
	return hexutil.DecodeBig(resp)
}

func (c *Client) Eth_GetStorageAt(address common.Address, key common.Hash) (common.Hash, error) {
	var (
		params = request.NewRawParams(address.String(), key.String())
		resp   = common.Hash{}
	)
	if err := c.performRequest("eth_getStorageAt", params, &resp); err != nil {
		return common.Hash{}, err
	}
	return resp, nil
}

func (c *Client) Eth_GetTransactionCount(address common.Address) (uint64, error) {
	var (
		params = request.NewRawParams(address.String())
		resp   = ""
	)
	if err := c.performRequest("eth_getTransactionCount", params, &resp); err != nil {
		return 0, err
	}
	return hexutil.DecodeUint64(resp)
}

func (c *Client) Eth_GetBlockTransactionCountByHash(blockHash common.Hash) (uint64, error) {
	var (
		params = request.NewRawParams(blockHash.String())
		resp   = ""
	)
	if err := c.performRequest("eth_getBlockTransactionCountByHash", params, &resp); err != nil {
		return 0, err
	}
	return hexutil.DecodeUint64(resp)
}

func (c *Client) Eth_GetBlockTransactionCountByNumber(height uint32) (uint64, error) {
	var (
		params = request.NewRawParams(hexutil.EncodeUint64(uint64(height)))
		resp   = ""
	)
	if err := c.performRequest("eth_getBlockTransactionCountByNumber", params, &resp); err != nil {
		return 0, err
	}
	return hexutil.DecodeUint64(resp)
}

func (c *Client) Eth_GetCode(address common.Address) ([]byte, error) {
	var (
		params = request.NewRawParams(address.String())
		resp   = ""
	)
	if err := c.performRequest("eth_getCode", params, &resp); err != nil {
		return nil, err
	}
	return hexutil.Decode(resp)
}

func (c *Client) Eth_Sign(address common.Address, msg []byte) ([]byte, error) {
	var (
		params = request.NewRawParams(address.String(), hexutil.Encode(msg))
		resp   = ""
	)
	if err := c.performRequest("eth_sign", params, &resp); err != nil {
		return nil, err
	}
	return hexutil.Decode(resp)
}

func (c *Client) Eth_SignTransaction(tx *result.TransactionObject) ([]byte, error) {
	var (
		params = request.NewRawParams(tx)
		resp   = ""
	)
	if err := c.performRequest("eth_signTransaction", params, &resp); err != nil {
		return nil, err
	}
	return hexutil.Decode(resp)
}

func (c *Client) Eth_SendTransaction(tx *result.TransactionObject) (common.Hash, error) {
	var (
		params = request.NewRawParams(tx)
		resp   = common.Hash{}
	)
	if err := c.performRequest("eth_sendTransaction", params, &resp); err != nil {
		return common.Hash{}, err
	}
	return resp, nil
}

func (c *Client) Eth_SendRawTransaction(raw []byte) (common.Hash, error) {
	var (
		params = request.NewRawParams(hexutil.Encode(raw))
		resp   = common.Hash{}
	)
	if err := c.performRequest("eth_sendRawTransaction", params, &resp); err != nil {
		return common.Hash{}, err
	}
	return resp, nil
}

func (c *Client) Eth_Call(tx *result.TransactionObject) ([]byte, error) {
	var (
		params = request.NewRawParams(tx)
		resp   = ""
	)
	if err := c.performRequest("eth_call", params, &resp); err != nil {
		return nil, err
	}
	return hexutil.Decode(resp)
}

func (c *Client) Eth_EstimateGas(tx *result.TransactionObject) (uint64, error) {
	var (
		params = request.NewRawParams(tx)
		resp   = ""
	)
	if err := c.performRequest("eth_estimateGas", params, &resp); err != nil {
		return 0, err
	}
	return hexutil.DecodeUint64(resp)
}

func (c *Client) Eth_GetBlockByHash(blockHash common.Hash) (*result.Block, error) {
	var (
		params = request.NewRawParams(blockHash.String())
		resp   = new(result.Block)
	)
	if err := c.performRequest("eth_getBlockByHash", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Eth_GetBlockByNumber(height uint32) (*result.Block, error) {
	var (
		params = request.NewRawParams(hexutil.EncodeUint64(uint64(height)))
		resp   = new(result.Block)
	)
	if err := c.performRequest("eth_getBlockByNumber", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Eth_GetTransactionByHash(txHash common.Hash) (*result.TransactionOutputRaw, error) {
	var (
		params = request.NewRawParams(txHash.String())
		resp   = new(result.TransactionOutputRaw)
	)
	if err := c.performRequest("eth_getTransactionByHash", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Eth_GetTransactionByBlockHashAndIndex(blockHash common.Hash, index int) (*result.TransactionOutputRaw, error) {
	var (
		params = request.NewRawParams(blockHash.String(), hexutil.EncodeUint64(uint64(index)))
		resp   = new(result.TransactionOutputRaw)
	)
	if err := c.performRequest("eth_getTransactionByBlockHashAndIndex", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Eth_GetTransactionByBlockNumberAndIndex(height uint32, index int) (*result.TransactionOutputRaw, error) {
	var (
		params = request.NewRawParams(hexutil.EncodeUint64(uint64(height)), hexutil.EncodeUint64(uint64(index)))
		resp   = new(result.TransactionOutputRaw)
	)
	if err := c.performRequest("eth_getTransactionByBlockNumberAndIndex", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Eth_GetTransactionReceipt(txHash common.Hash) (*types.Receipt, error) {
	var (
		params = request.NewRawParams(txHash.String())
		resp   = new(types.Receipt)
	)
	if err := c.performRequest("eth_getTransactionReceipt", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Eth_GetLogs(filter *filters.LogFilter) ([]*types.Log, error) {
	var (
		params = request.NewRawParams(filter)
		resp   = []*types.Log{}
	)
	if err := c.performRequest("eth_getLogs", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
