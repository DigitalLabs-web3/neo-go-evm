package client

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/request"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response/result"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/wallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var errNetworkNotInitialized = errors.New("RPC client network is not initialized")

func (c *Client) IsBlocked(address common.Address) (bool, error) {
	resp := false
	if err := c.performRequest("isblocked", request.NewRawParams(address.String()), &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *Client) CalculateGas(tx *transaction.Transaction) (uint64, error) {
	b, err := tx.Bytes()
	if err != nil {
		return 0, err
	}
	var (
		params = request.NewRawParams(hexutil.Bytes(b))
		resp   = new(result.NetworkFee)
	)
	if err := c.performRequest("calculategas", params, resp); err != nil {
		return 0, err
	}
	return resp.Value, nil
}

// GetBestBlockHash returns the hash of the tallest block in the main chain.
func (c *Client) GetBestBlockHash() (common.Hash, error) {
	var resp = common.Hash{}
	if err := c.performRequest("getbestblockhash", request.NewRawParams(), &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockCount returns the number of blocks in the main chain.
func (c *Client) GetBlockCount() (uint32, error) {
	var resp uint32
	if err := c.performRequest("getblockcount", request.NewRawParams(), &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockByIndex returns a block by its height. You should initialize network magic
// with Init before calling GetBlockByIndex.
func (c *Client) GetBlockByIndex(index uint32) (*block.Block, error) {
	return c.getBlock(request.NewRawParams(index))
}

// GetBlockByHash returns a block by its hash. You should initialize network magic
// with Init before calling GetBlockByHash.
func (c *Client) GetBlockByHash(hash common.Hash) (*block.Block, error) {
	return c.getBlock(request.NewRawParams(hash.String()))
}

func (c *Client) getBlock(params request.RawParams) (*block.Block, error) {
	var (
		resp []byte
		err  error
		b    *block.Block
	)
	if err = c.performRequest("getblock", params, &resp); err != nil {
		return nil, err
	}
	b = new(block.Block)
	err = io.FromByteArray(b, resp)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GetBlockByIndexVerbose returns a block wrapper with additional metadata by
// its height. You should initialize network magic with Init before calling GetBlockByIndexVerbose.
// NOTE: to get transaction.ID and transaction.Size, use t.Hash() and io.GetVarSize(t) respectively.
func (c *Client) GetBlockByIndexVerbose(index uint32) (*result.Block, error) {
	return c.getBlockVerbose(request.NewRawParams(index, 1))
}

// GetBlockByHashVerbose returns a block wrapper with additional metadata by
// its hash. You should initialize network magic with Init before calling GetBlockByHashVerbose.
func (c *Client) GetBlockByHashVerbose(hash common.Hash) (*result.Block, error) {
	return c.getBlockVerbose(request.NewRawParams(hash.String(), 1))
}

func (c *Client) getBlockVerbose(params request.RawParams) (*result.Block, error) {
	var (
		resp = &result.Block{}
		err  error
	)
	if err = c.performRequest("getblock", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockHash returns the hash value of the corresponding block, based on the specified index.
func (c *Client) GetBlockHash(index uint32) (common.Hash, error) {
	var (
		params = request.NewRawParams(index)
		resp   = common.Hash{}
	)
	if err := c.performRequest("getblockhash", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockHeader returns the corresponding block header information from serialized hex string
// according to the specified script hash. You should initialize network magic
// // with Init before calling GetBlockHeader.
func (c *Client) GetBlockHeader(hash common.Hash) (*block.Header, error) {
	var (
		params = request.NewRawParams(hash.String())
		resp   []byte
		h      *block.Header
	)
	if err := c.performRequest("getblockheader", params, &resp); err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(resp)
	h = new(block.Header)
	h.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return h, nil
}

// GetBlockHeaderCount returns the number of headers in the main chain.
func (c *Client) GetBlockHeaderCount() (uint32, error) {
	var resp uint32
	if err := c.performRequest("getblockheadercount", request.NewRawParams(), &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockHeaderVerbose returns the corresponding block header information from Json format string
// according to the specified script hash.
func (c *Client) GetBlockHeaderVerbose(hash common.Hash) (*result.Header, error) {
	var (
		params = request.NewRawParams(hash.String(), 1)
		resp   = &result.Header{}
	)
	if err := c.performRequest("getblockheader", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockSysFee returns the system fees of the block, based on the specified index.
func (c *Client) GetBlockGas(index uint32) (uint64, error) {
	var (
		params = request.NewRawParams(index)
		resp   uint64
	)
	if err := c.performRequest("getblockgas", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetConnectionCount returns the current number of connections for the node.
func (c *Client) GetConnectionCount() (int, error) {
	var (
		params = request.NewRawParams()
		resp   int
	)
	if err := c.performRequest("getconnectioncount", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *Client) GetConsensusAddress() (common.Address, error) {
	var (
		params = request.NewRawParams()
		resp   = new(common.Address)
	)
	if err := c.performRequest("getconsensusaddress", params, resp); err != nil {
		return common.Address{}, err
	}
	return *resp, nil
}

func (c *Client) GetValidators(index uint32) (keys.PublicKeys, error) {
	var (
		params = request.NewRawParams(index)
		resp   = new(keys.PublicKeys)
	)
	if err := c.performRequest("getnextvalidators", params, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

func (c *Client) GetNextValidators() (keys.PublicKeys, error) {
	var (
		params = request.NewRawParams()
		resp   = new(keys.PublicKeys)
	)
	if err := c.performRequest("getnextvalidators", params, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

// GetContractStateByHash queries contract information, according to the contract script hash.
func (c *Client) GetContractStateByHash(hash common.Address) (*state.Contract, error) {
	return c.getContractState(hash.String())
}

// GetContractStateByAddressOrName queries contract information, according to the contract address or name.
func (c *Client) GetContractStateByAddressOrName(addressOrName string) (*state.Contract, error) {
	return c.getContractState(addressOrName)
}

// GetContractStateByID queries contract information, according to the contract ID.
func (c *Client) GetContractStateByID(id int32) (*state.Contract, error) {
	return c.getContractState(id)
}

// getContractState is an internal representation of GetContractStateBy* methods.
func (c *Client) getContractState(param interface{}) (*state.Contract, error) {
	var (
		params = request.NewRawParams(param)
		resp   = &state.Contract{}
	)
	if err := c.performRequest("getcontractstate", params, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *Client) GetFeePerByte() (uint64, error) {
	var (
		params = request.NewRawParams()
		resp   = uint64(0)
	)
	if err := c.performRequest("getfeeperbyte", params, &resp); err != nil {
		return 0, err
	}
	return resp, nil
}

// GetNativeContracts queries information about native contracts.
func (c *Client) GetNativeContracts() ([]state.NativeContract, error) {
	var (
		params = request.NewRawParams()
		resp   []state.NativeContract
	)
	if err := c.performRequest("getnativecontracts", params, &resp); err != nil {
		return resp, err
	}

	// Update native contract hashes.
	c.cacheLock.Lock()
	for _, cs := range resp {
		c.cache.nativeHashes[cs.Name] = cs.Address
	}
	c.cacheLock.Unlock()

	return resp, nil
}

// GetPeers returns the list of nodes that the node is currently connected/disconnected from.
func (c *Client) GetPeers() (*result.GetPeers, error) {
	var (
		params = request.NewRawParams()
		resp   = &result.GetPeers{}
	)
	if err := c.performRequest("getpeers", params, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetRawMemPool returns the list of unconfirmed transactions in memory.
func (c *Client) GetRawMemPool() ([]common.Hash, error) {
	var (
		params = request.NewRawParams()
		resp   = new([]common.Hash)
	)
	if err := c.performRequest("getrawmempool", params, resp); err != nil {
		return *resp, err
	}
	return *resp, nil
}

// GetRawTransaction returns a transaction by hash.
func (c *Client) GetRawTransaction(hash common.Hash) (*transaction.Transaction, error) {
	var (
		params = request.NewRawParams(hash.String())
		resp   []byte
		err    error
	)
	if err = c.performRequest("getrawtransaction", params, &resp); err != nil {
		return nil, err
	}
	tx := new(transaction.Transaction)
	err = io.FromByteArray(tx, resp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// GetRawTransactionVerbose returns a transaction wrapper with additional
// metadata by transaction's hash.
// NOTE: to get transaction.ID and transaction.Size, use t.Hash() and io.GetVarSize(t) respectively.
func (c *Client) GetRawTransactionVerbose(hash common.Hash) (*result.TransactionOutputRaw, error) {
	var (
		params = request.NewRawParams(hash.String(), 1)
		resp   = &result.TransactionOutputRaw{}
		err    error
	)
	if err = c.performRequest("getrawtransaction", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) GetProof(stateroot common.Hash, historicalContractHash common.Address, historicalKey []byte) (*result.ProofWithKey, error) {
	var (
		params = request.NewRawParams(stateroot.String(), historicalContractHash.String(), hex.EncodeToString(historicalKey))
		resp   = new(result.ProofWithKey)
	)
	if err := c.performRequest("getproof", params, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetState returns historical contract storage item state by the given stateroot,
// historical contract hash and historical item key.
func (c *Client) GetState(stateroot common.Hash, historicalContractHash common.Address, historicalKey []byte) ([]byte, error) {
	var (
		params = request.NewRawParams(stateroot.String(), historicalContractHash.String(), historicalKey)
		resp   []byte
	)
	if err := c.performRequest("getstate", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// FindStates returns historical contract storage item states by the given stateroot,
// historical contract hash and historical prefix. If `start` path is specified, then items
// starting from `start` path are being returned (excluding item located at the start path).
// If `maxCount` specified, then maximum number of items to be returned equals to `maxCount`.
func (c *Client) FindStates(stateroot common.Hash, historicalContractHash common.Address, historicalPrefix []byte,
	start []byte, maxCount *int) (result.FindStates, error) {
	if historicalPrefix == nil {
		historicalPrefix = []byte{}
	}
	var (
		params = request.NewRawParams(stateroot.String(), historicalContractHash.String(), historicalPrefix)
		resp   result.FindStates
	)
	if start == nil && maxCount != nil {
		start = []byte{}
	}
	if start != nil {
		params.Values = append(params.Values, start)
	}
	if maxCount != nil {
		params.Values = append(params.Values, *maxCount)
	}
	if err := c.performRequest("findstates", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetStateRootByHeight returns state root for the specified height.
func (c *Client) GetStateRootByHeight(height uint32) (*state.MPTRoot, error) {
	return c.getStateRoot(request.NewRawParams(height))
}

// GetStateRootByBlockHash returns state root for block with specified hash.
func (c *Client) GetStateRootByBlockHash(hash common.Hash) (*state.MPTRoot, error) {
	return c.getStateRoot(request.NewRawParams(hash))
}

func (c *Client) getStateRoot(params request.RawParams) (*state.MPTRoot, error) {
	var resp = []byte{}
	if err := c.performRequest("getstateroot", params, &resp); err != nil {
		return nil, err
	}
	sr := new(state.MPTRoot)
	err := io.FromByteArray(sr, resp)
	if err != nil {
		return nil, err
	}
	return sr, nil
}

// GetStateHeight returns current validated and local node state height.
func (c *Client) GetStateHeight() (*result.StateHeight, error) {
	var (
		params = request.NewRawParams()
		resp   = new(result.StateHeight)
	)
	if err := c.performRequest("getstateheight", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetStorageByID returns the stored value, according to the contract ID and the stored key.
func (c *Client) GetStorageByID(id int32, key []byte) ([]byte, error) {
	return c.getStorage(request.NewRawParams(id, key))
}

// GetStorageByHash returns the stored value, according to the contract script hash and the stored key.
func (c *Client) GetStorageByHash(hash common.Address, key []byte) ([]byte, error) {
	return c.getStorage(request.NewRawParams(hash.String(), key))
}

func (c *Client) getStorage(params request.RawParams) ([]byte, error) {
	var resp []byte
	if err := c.performRequest("getstorage", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetTransactionHeight returns the block index in which the transaction is found.
func (c *Client) GetTransactionHeight(hash common.Hash) (uint32, error) {
	var (
		params = request.NewRawParams(hash.String())
		resp   uint32
	)
	if err := c.performRequest("gettransactionheight", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetUnclaimedGas returns unclaimed GAS amount for the specified address.
func (c *Client) GetUnclaimedGas(address string) (result.UnclaimedGas, error) {
	var (
		params = request.NewRawParams(address)
		resp   result.UnclaimedGas
	)
	if err := c.performRequest("getunclaimedgas", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *Client) GetNextBlockValidators() ([]result.Validator, error) {
	var (
		params = request.NewRawParams()
		resp   = new([]result.Validator)
	)
	if err := c.performRequest("getnextblockvalidators", params, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

// GetVersion returns the version information about the queried node.
func (c *Client) GetVersion() (*result.Version, error) {
	var (
		params = request.NewRawParams()
		resp   = &result.Version{}
	)
	if err := c.performRequest("getversion", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) SendRawTransaction(rawTX []byte) (common.Hash, error) {
	var (
		params = request.NewRawParams(hexutil.Bytes(rawTX))
		resp   = &common.Hash{}
	)
	if err := c.performRequest("sendrawtransaction", params, resp); err != nil {
		return common.Hash{}, err
	}
	return *resp, nil
}

func (c *Client) SubmitBlock(b block.Block) (common.Hash, error) {
	var (
		params request.RawParams
		resp   = new(result.RelayResult)
	)
	buf := io.NewBufBinWriter()
	b.EncodeBinary(buf.BinWriter)
	if err := buf.Err; err != nil {
		return common.Hash{}, err
	}
	params = request.NewRawParams(buf.Bytes())

	if err := c.performRequest("submitblock", params, resp); err != nil {
		return common.Hash{}, err
	}
	return resp.Hash, nil
}

// SignAndPushTx signs given transaction using given wif and cosigners and pushes
// it to the chain. It returns a hash of the transaction and an error. If one of
// the cosigners accounts is neither contract-based nor unlocked an error is
// returned.
func (c *Client) SignAndPushTx(tx *transaction.Transaction, acc *wallet.Account) (common.Hash, error) {
	var (
		txHash common.Hash
		err    error
	)
	m, err := c.GetNetwork()
	if err != nil {
		return txHash, fmt.Errorf("failed to sign tx: %w", err)
	}
	if err = acc.SignTx(m, tx); err != nil {
		return txHash, fmt.Errorf("failed to sign tx: %w", err)
	}
	txHash = tx.Hash()
	b, err := tx.Bytes()
	if err != nil {
		return common.Hash{}, err
	}
	actualHash, err := c.SendRawTransaction(b)
	if err != nil {
		return txHash, fmt.Errorf("failed to send tx: %w", err)
	}
	if actualHash != txHash {
		return actualHash, fmt.Errorf("sent and actual tx hashes mismatch:\n\tsent: %v\n\tactual: %v", txHash.String(), actualHash.String())
	}
	return txHash, nil
}

func (c *Client) ValidateAddress(address string) error {
	var (
		params = request.NewRawParams(address)
		resp   = &result.ValidateAddress{}
	)

	if err := c.performRequest("validateaddress", params, resp); err != nil {
		return err
	}
	if !resp.IsValid {
		return errors.New("validateaddress returned false")
	}
	return nil
}

// GetNetwork returns the network magic of the RPC node client connected to.
func (c *Client) GetNetwork() (uint64, error) {
	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()

	if !c.cache.initDone {
		return 0, errNetworkNotInitialized
	}
	return c.cache.chainId, nil
}

// GetNativeContractHash returns native contract hash by its name.
func (c *Client) GetNativeContractHash(name string) (common.Address, error) {
	c.cacheLock.RLock()
	hash, ok := c.cache.nativeHashes[name]
	c.cacheLock.RUnlock()
	if ok {
		return hash, nil
	}
	cs, err := c.GetContractStateByAddressOrName(name)
	if err != nil {
		return common.Address{}, err
	}
	c.cacheLock.Lock()
	c.cache.nativeHashes[name] = cs.Address
	c.cacheLock.Unlock()
	return cs.Address, nil
}
