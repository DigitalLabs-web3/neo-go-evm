package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/blockchainer"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/filters"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/mpt"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/network"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/request"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response/result"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/wallet"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		chain      blockchainer.Blockchainer
		config     rpc.Config
		chainId    uint64
		coreServer *network.Server
		log        *zap.Logger
		https      *http.Server
		shutdown   chan struct{}

		subsLock         sync.RWMutex
		subscribers      map[*subscriber]bool
		blockSubs        int
		executionSubs    int
		notificationSubs int
		transactionSubs  int
		blockCh          chan *block.Block
		executionCh      chan *types.Receipt
		notificationCh   chan *types.Log
		transactionCh    chan *transaction.Transaction

		accounts []*wallet.Account
	}
)

const (
	// Message limit for receiving side.
	wsReadLimit = 4096

	// Disconnection timeout.
	wsPongLimit = 60 * time.Second

	// Ping period for connection liveness check.
	wsPingPeriod = wsPongLimit / 2

	// Write deadline.
	wsWriteLimit = wsPingPeriod / 2

	// Maximum number of subscribers per Server. Each websocket client is
	// treated like subscriber, so technically it's a limit on websocket
	// connections.
	maxSubscribers = 64

	TestGas uint64 = 250000000
)

var rpcHandlers = map[string]func(*Server, request.Params) (interface{}, *response.Error){
	// -- start eth api
	"web3_clientVersion":                      (*Server).web3_clientVersion,
	"web3_sha3":                               (*Server).web3_sha3,
	"net_version":                             (*Server).net_version,
	"net_peerCount":                           (*Server).net_peerCount,
	"net_listening":                           (*Server).net_listening,
	"eth_protocolVersion":                     (*Server).eth_protocolVersion,
	"eth_chainId":                             (*Server).eth_chainId,
	"eth_syncing":                             (*Server).eth_syncing,
	"eth_gasPrice":                            (*Server).eth_gasPrice,
	"eth_accounts":                            (*Server).eth_accounts,
	"eth_blockNumber":                         (*Server).eth_blockNumber,
	"eth_getBalance":                          (*Server).eth_getBalance,
	"eth_getStorageAt":                        (*Server).eth_getStorageAt,
	"eth_getTransactionCount":                 (*Server).eth_getTransactionCount,
	"eth_getBlockTransactionCountByHash":      (*Server).eth_getBlockTransactionCountByHash,
	"eth_getBlockTransactionCountByNumber":    (*Server).eth_getBlockTransactionCountByNumber,
	"eth_getCode":                             (*Server).eth_getCode,
	"eth_sign":                                (*Server).eth_sign,
	"eth_signTransaction":                     (*Server).eth_signTransaction,
	"eth_sendTransaction":                     (*Server).eth_sendTransaction,
	"eth_sendRawTransaction":                  (*Server).eth_sendRawTransaction,
	"eth_call":                                (*Server).eth_call,
	"eth_estimateGas":                         (*Server).eth_estimateGas,
	"eth_getBlockByHash":                      (*Server).eth_getBlockByHash,
	"eth_getBlockByNumber":                    (*Server).eth_getBlockByNumber,
	"eth_getTransactionByHash":                (*Server).eth_getTransactionByHash,
	"eth_getTransactionByBlockHashAndIndex":   (*Server).eth_getTransactionByBlockHashAndIndex,
	"eth_getTransactionByBlockNumberAndIndex": (*Server).eth_getTransactionByBlockNumberAndIndex,
	"eth_getTransactionReceipt":               (*Server).eth_getTransactionReceipt,
	"eth_newFilter":                           (*Server).eth_newFilter,
	"eth_newBlockFilter":                      (*Server).eth_newBlockFilter,
	"eth_newPendingTransactionFilter":         (*Server).eth_newPendingTransactionFilter,
	"eth_uninstallFilter":                     (*Server).eth_uninstallFilter,
	"eth_getFilterChanges":                    (*Server).eth_getFilterChanges,
	"eth_getFilterLogs":                       (*Server).eth_getFilterLogs,
	"eth_getLogs":                             (*Server).eth_getLogs,
	"eth_getUncleByBlockHashAndIndex":         (*Server).eth_getUncleByBlockHashAndIndex,
	"eth_feeHistory":                          (*Server).eth_feeHistory,
	// -- end eth api

	// -- start trace api
	"trace_call": (*Server).trace_call,
	// -- end trace api

	// -- start gether api
	"txpool_content": (*Server).txpool_content,
	// -- end gether api

	// -- start bridge api
	"bridge_getMinted": (*Server).getMinted,
	// -- end bridge api

	// -- start neo api
	"getversion":           (*Server).getVersion,
	"calculategas":         (*Server).calculateGas,
	"findstates":           (*Server).findStates,
	"getbestblockhash":     (*Server).getBestBlockHash,
	"getblock":             (*Server).getBlock,
	"getblockcount":        (*Server).getBlockCount,
	"getblockhash":         (*Server).getBlockHash,
	"getblockheader":       (*Server).getBlockHeader,
	"getblockheadercount":  (*Server).getBlockHeaderCount,
	"getblocksysfee":       (*Server).getBlockGas,
	"getconsensusaddress":  (*Server).getConsensusAddress,
	"getconnectioncount":   (*Server).getConnectionCount,
	"getcontractstate":     (*Server).getContractState,
	"getfeeperbyte":        (*Server).getFeePerByte,
	"getnativecontracts":   (*Server).getNativeContracts,
	"getpeers":             (*Server).getPeers,
	"getproof":             (*Server).getProof,
	"getrawmempool":        (*Server).getRawMempool,
	"getrawtransaction":    (*Server).getrawtransaction,
	"getstate":             (*Server).getState,
	"getstateheight":       (*Server).getStateHeight,
	"getstateroot":         (*Server).getStateRoot,
	"getstorage":           (*Server).getStorage,
	"gettransactionheight": (*Server).getTransactionHeight,
	"getvalidators":        (*Server).getValidators,
	"getnextvalidators":    (*Server).getNextValidators,
	"sendrawtransaction":   (*Server).sendrawtransaction,
	"validateaddress":      (*Server).validateAddress,
	"verifyproof":          (*Server).verifyProof,
	"isblocked":            (*Server).isBlocked,
}

var rpcWsHandlers = map[string]func(*Server, request.Params, *subscriber) (interface{}, *response.Error){
	"subscribe":   (*Server).subscribe,
	"unsubscribe": (*Server).unsubscribe,
}

var invalidBlockHeightError = func(index int, height int) *response.Error {
	return response.NewRPCError(fmt.Sprintf("Param at index %d should be greater than or equal to 0 and less then or equal to current block height, got: %d", index, height), "", nil)
}

// upgrader is a no-op websocket.Upgrader that reuses HTTP server buffers and
// doesn't set any Error function.
var upgrader = websocket.Upgrader{}

// New creates a new Server struct.
func New(chain blockchainer.Blockchainer, conf rpc.Config, coreServer *network.Server, wall *config.Wallet, log *zap.Logger) Server {
	httpServer := &http.Server{
		Addr: conf.Address + ":" + strconv.FormatUint(uint64(conf.Port), 10),
	}

	var tlsServer *http.Server
	if cfg := conf.TLSConfig; cfg.Enabled {
		tlsServer = &http.Server{
			Addr: net.JoinHostPort(cfg.Address, strconv.FormatUint(uint64(cfg.Port), 10)),
		}
	}

	return Server{
		Server:     httpServer,
		chain:      chain,
		config:     conf,
		chainId:    chain.GetConfig().ChainID,
		coreServer: coreServer,
		log:        log,
		https:      tlsServer,
		shutdown:   make(chan struct{}),

		subscribers: make(map[*subscriber]bool),
		// These are NOT buffered to preserve original order of events.
		blockCh:        make(chan *block.Block),
		executionCh:    make(chan *types.Receipt),
		notificationCh: make(chan *types.Log),
		transactionCh:  make(chan *transaction.Transaction),

		accounts: getAccounts(wall),
	}
}

func getAccounts(wall *config.Wallet) []*wallet.Account {
	if wall == nil {
		return nil
	}
	accs := []*wallet.Account{nil}
	w, err := wallet.NewWalletFromFile(wall.Path)
	if err != nil {
		return nil
	}
	for _, acc := range w.Accounts {
		err := acc.Decrypt(wall.Password, w.Scrypt)
		if err == nil {
			if acc.Default && accs[0] == nil {
				accs[0] = acc
			} else {
				accs = append(accs, acc)
			}
		}
	}
	if accs[0] == nil {
		accs = accs[1:]
	}
	if len(accs) == 0 {
		return nil
	}
	return accs
}

// Start creates a new JSON-RPC server listening on the configured port. It's
// supposed to be run as a separate goroutine (like http.Server's Serve) and it
// returns its errors via given errChan.
func (s *Server) Start(errChan chan error) {
	if !s.config.Enabled {
		s.log.Info("RPC server is not enabled")
		return
	}
	s.Handler = http.HandlerFunc(s.handleHTTPRequest)
	s.log.Info("starting rpc-server", zap.String("endpoint", s.Addr))

	go s.handleSubEvents()
	if cfg := s.config.TLSConfig; cfg.Enabled {
		s.https.Handler = http.HandlerFunc(s.handleHTTPRequest)
		s.log.Info("starting rpc-server (https)", zap.String("endpoint", s.https.Addr))
		go func() {
			ln, err := net.Listen("tcp", s.https.Addr)
			if err != nil {
				errChan <- err
				return
			}
			s.https.Addr = ln.Addr().String()
			err = s.https.ServeTLS(ln, cfg.CertFile, cfg.KeyFile)
			if err != http.ErrServerClosed {
				s.log.Error("failed to start TLS RPC server", zap.Error(err))
				errChan <- err
			}
		}()
	}
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		errChan <- err
		return
	}
	s.Addr = ln.Addr().String() // set Addr to the actual address
	go func() {
		err = s.Serve(ln)
		if err != http.ErrServerClosed {
			s.log.Error("failed to start RPC server", zap.Error(err))
			errChan <- err
		}
	}()
}

// Shutdown overrides the http.Server Shutdown
// method.
func (s *Server) Shutdown() error {
	if !s.config.Enabled {
		return nil
	}
	var httpsErr error

	// Signal to websocket writer routines and handleSubEvents.
	close(s.shutdown)

	if s.config.TLSConfig.Enabled {
		s.log.Info("shutting down rpc-server (https)", zap.String("endpoint", s.https.Addr))
		httpsErr = s.https.Shutdown(context.Background())
	}

	s.log.Info("shutting down rpc-server", zap.String("endpoint", s.Addr))
	err := s.Server.Shutdown(context.Background())

	// Wait for handleSubEvents to finish.
	<-s.executionCh

	if err == nil {
		return httpsErr
	}
	return err
}

func (s *Server) handleHTTPRequest(w http.ResponseWriter, httpRequest *http.Request) {
	req := request.NewRequest()

	if httpRequest.URL.Path == "/ws" && httpRequest.Method == "GET" {
		// Technically there is a race between this check and
		// s.subscribers modification 20 lines below, but it's tiny
		// and not really critical to bother with it. Some additional
		// clients may sneak in, no big deal.
		s.subsLock.RLock()
		numOfSubs := len(s.subscribers)
		s.subsLock.RUnlock()
		if numOfSubs >= maxSubscribers {
			s.writeHTTPErrorResponse(
				request.NewIn(),
				w,
				response.NewInternalServerError("websocket users limit reached", nil),
			)
			return
		}
		ws, err := upgrader.Upgrade(w, httpRequest, nil)
		if err != nil {
			s.log.Info("websocket connection upgrade failed", zap.Error(err))
			return
		}
		resChan := make(chan response.AbstractResult) // response.Abstract or response.AbstractBatch
		subChan := make(chan *websocket.PreparedMessage, notificationBufSize)
		subscr := &subscriber{writer: subChan, ws: ws}
		s.subsLock.Lock()
		s.subscribers[subscr] = true
		s.subsLock.Unlock()
		go s.handleWsWrites(ws, resChan, subChan)
		s.handleWsReads(ws, resChan, subscr)
		return
	}

	if httpRequest.Method != "POST" {
		s.writeHTTPErrorResponse(
			request.NewIn(),
			w,
			response.NewInvalidParamsError(
				fmt.Sprintf("Invalid method '%s', please retry with 'POST'", httpRequest.Method), nil,
			),
		)
		return
	}

	err := req.DecodeData(httpRequest.Body)
	if err != nil {
		s.writeHTTPErrorResponse(request.NewIn(), w, response.NewParseError("Problem parsing JSON-RPC request body", err))
		return
	}

	resp := s.handleRequest(req, nil)
	s.writeHTTPServerResponse(req, w, resp)
}

func (s *Server) handleRequest(req *request.Request, sub *subscriber) response.AbstractResult {
	if req.In != nil {
		return s.handleIn(req.In, sub)
	}
	resp := make(response.AbstractBatch, len(req.Batch))
	for i, in := range req.Batch {
		resp[i] = s.handleIn(&in, sub)
	}
	return resp
}

func (s *Server) handleIn(req *request.In, sub *subscriber) response.Abstract {
	var res interface{}
	var resErr *response.Error
	if req.JSONRPC != request.JSONRPCVersion {
		return s.packResponse(req, nil, response.NewInvalidParamsError("Problem parsing JSON", fmt.Errorf("invalid version, expected 2.0 got: '%s'", req.JSONRPC)))
	}

	reqParams := request.Params(req.RawParams)

	s.log.Debug("processing rpc request",
		zap.String("method", req.Method),
		zap.Stringer("params", reqParams))

	incCounter(req.Method)

	resErr = response.NewMethodNotFoundError(fmt.Sprintf("Method '%s' not supported", req.Method), nil)
	handler, ok := rpcHandlers[req.Method]
	if ok {
		res, resErr = handler(s, reqParams)
	} else if sub != nil {
		handler, ok := rpcWsHandlers[req.Method]
		if ok {
			res, resErr = handler(s, reqParams, sub)
		}
	}
	return s.packResponse(req, res, resErr)
}

func (s *Server) handleWsWrites(ws *websocket.Conn, resChan <-chan response.AbstractResult, subChan <-chan *websocket.PreparedMessage) {
	pingTicker := time.NewTicker(wsPingPeriod)
eventloop:
	for {
		select {
		case <-s.shutdown:
			break eventloop
		case event, ok := <-subChan:
			if !ok {
				break eventloop
			}
			if err := ws.SetWriteDeadline(time.Now().Add(wsWriteLimit)); err != nil {
				break eventloop
			}
			if err := ws.WritePreparedMessage(event); err != nil {
				break eventloop
			}
		case res, ok := <-resChan:
			if !ok {
				break eventloop
			}
			if err := ws.SetWriteDeadline(time.Now().Add(wsWriteLimit)); err != nil {
				break eventloop
			}
			if err := ws.WriteJSON(res); err != nil {
				break eventloop
			}
		case <-pingTicker.C:
			if err := ws.SetWriteDeadline(time.Now().Add(wsWriteLimit)); err != nil {
				break eventloop
			}
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				break eventloop
			}
		}
	}
	ws.Close()
	pingTicker.Stop()
	// Drain notification channel as there might be some goroutines blocked
	// on it.
drainloop:
	for {
		select {
		case _, ok := <-subChan:
			if !ok {
				break drainloop
			}
		default:
			break drainloop
		}
	}
}

func (s *Server) handleWsReads(ws *websocket.Conn, resChan chan<- response.AbstractResult, subscr *subscriber) {
	ws.SetReadLimit(wsReadLimit)
	err := ws.SetReadDeadline(time.Now().Add(wsPongLimit))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(wsPongLimit)) })
requestloop:
	for err == nil {
		req := request.NewRequest()
		err := ws.ReadJSON(req)
		if err != nil {
			break
		}
		res := s.handleRequest(req, subscr)
		res.RunForErrors(func(jsonErr *response.Error) {
			s.logRequestError(req, jsonErr)
		})
		select {
		case <-s.shutdown:
			break requestloop
		case resChan <- res:
		}
	}
	s.subsLock.Lock()
	delete(s.subscribers, subscr)
	for _, e := range subscr.feeds {
		if e.event != response.InvalidEventID {
			s.unsubscribeFromChannel(e.event)
		}
	}
	s.subsLock.Unlock()
	close(resChan)
	ws.Close()
}

// -- start eth api.

func (s *Server) getWalletAccount(address common.Address) *wallet.Account {
	if s.accounts == nil {
		return nil
	}
	for _, account := range s.accounts {
		if account.Address == address {
			return account
		}
	}
	return nil
}

func (s *Server) web3_clientVersion(_ request.Params) (interface{}, *response.Error) {
	return s.coreServer.UserAgent, nil
}

func (s *Server) web3_sha3(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	bs, err := param.GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	data, err := hexutil.Decode(bs)
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	return hash.Keccak256(data), nil
}

func (s *Server) net_version(_ request.Params) (interface{}, *response.Error) {
	return hexutil.EncodeUint64(s.chainId), nil
}

func (s *Server) net_peerCount(_ request.Params) (interface{}, *response.Error) {
	return hexutil.EncodeUint64(uint64(s.coreServer.PeerCount())), nil
}

func (s *Server) net_listening(_ request.Params) (interface{}, *response.Error) {
	return s.coreServer != nil, nil
}

func (s *Server) eth_protocolVersion(_ request.Params) (interface{}, *response.Error) {
	return strconv.Itoa(int(s.chainId)), nil
}

func (s *Server) eth_chainId(_ request.Params) (interface{}, *response.Error) {
	return hexutil.EncodeUint64(s.chainId), nil
}

func (s *Server) eth_syncing(_ request.Params) (interface{}, *response.Error) {
	return result.Syncing{
		StartingBlock: "0x0",
		CurrentBlock:  hexutil.EncodeUint64(uint64(s.chain.BlockHeight())),
		HighestBlock:  hexutil.EncodeUint64(uint64(s.chain.HeaderHeight())),
	}, nil
}

func (s *Server) eth_gasPrice(_ request.Params) (interface{}, *response.Error) {
	return hexutil.EncodeBig(s.chain.GetGasPrice()), nil
}

func (s *Server) eth_accounts(_ request.Params) (interface{}, *response.Error) {
	result := make([]common.Address, len(s.accounts))
	for i, acc := range s.accounts {
		result[i] = acc.Address
	}
	return result, nil
}

func (s *Server) eth_blockNumber(_ request.Params) (interface{}, *response.Error) {
	return hexutil.EncodeUint64(uint64(s.chain.BlockHeight())), nil
}

// We only return latest balance of GAS
func (s *Server) eth_getBalance(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	addr, err := param.GetAddressFromHex()
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}
	balance := s.chain.GetUtilityTokenBalance(addr)
	return hexutil.EncodeBig(balance), nil
}

func (s *Server) eth_getStorageAt(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	addr, err := param.GetAddressFromHex()
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}
	param = params.Value(1)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	key, err := param.GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	hashKey := common.HexToHash(key)
	si := s.chain.GetStorageItem(addr, hashKey.Bytes())
	return hexutil.Encode(common.BytesToHash(si).Bytes()), nil
}

func (s *Server) eth_getTransactionCount(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	addr, err := param.GetAddressFromHex()
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}
	return hexutil.EncodeUint64(s.chain.GetNonce(addr)), nil
}

func (s *Server) eth_getBlockTransactionCountByHash(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	hash, err := param.GetHash()
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}
	b, _, err := s.chain.GetBlock(hash, false)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
	}
	return hexutil.EncodeUint64(uint64(len(b.Transactions))), nil
}

func (s *Server) eth_getBlockTransactionCountByNumber(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	num, err := param.GetString()
	if err != nil {
		return nil, response.NewInvalidParamsError(fmt.Sprintf("invalid number: %s", err), err)
	}
	index, err := hexutil.DecodeUint64(num)
	if err != nil {
		return nil, response.NewInvalidParamsError(fmt.Sprintf("Problem parsing block number: %s", num), err)
	}
	hash := s.chain.GetHeaderHash(int(index))
	if hash == (common.Hash{}) {
		return nil, nil
	}
	b, _, err := s.chain.GetBlock(hash, false)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
	}
	return hexutil.EncodeUint64(uint64(len(b.Transactions))), nil
}

func (s *Server) eth_getCode(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	addr, err := param.GetAddressFromHex()
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}
	cs := s.chain.GetContractState(addr)
	if cs == nil {
		return nil, nil
	}
	return hexutil.Encode(cs.Code), nil
}

func (s *Server) eth_sign(params request.Params) (interface{}, *response.Error) {
	if s.accounts == nil {
		return nil, response.NewInternalServerError("No wallet opened", errors.New("wallet not open"))
	}
	saddr, err := params.Value(0).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	addr := common.HexToAddress(saddr)
	var acc *wallet.Account
	for _, a := range s.accounts {
		if a.Address == addr {
			acc = a
			break
		}
	}
	if acc == nil {
		return nil, response.NewInternalServerError("Account not found", errors.New("account not found"))
	}
	text, err := params.Value(1).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	data, err := hexutil.Decode(text)
	if err != nil {
		return nil, response.NewInvalidParamsError(fmt.Sprintf("Could not decode hex text: %s", err), err)
	}
	hash := accounts.TextHash(data)
	sig, err := crypto.Sign(hash, &acc.PrivateKey().PrivateKey)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Failed sign tx: %s", err), err)
	}
	return hexutil.Encode(sig), nil
}

func (s *Server) eth_signTransaction(params request.Params) (interface{}, *response.Error) {
	param, err := params.Value(0).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	data := []byte(param)
	txObj := result.TransactionObject{}
	err = json.Unmarshal(data, &txObj)
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	acc := s.getWalletAccount(txObj.From)
	if acc == nil {
		return nil, response.NewInternalServerError("Could not found account to sign tx", errors.New("account not found"))
	}
	ltx := &types.LegacyTx{
		Nonce: s.chain.GetNonce(txObj.From) + 1,
		To:    txObj.To,
		Value: txObj.Value,
		Data:  txObj.Data,
	}
	inner := &transaction.EthTx{
		Transaction: *types.NewTx(ltx),
		ChainID:     s.chainId,
		Sender:      txObj.From,
	}
	tx := transaction.NewTx(inner)
	fakeBlock, _, err := s.chain.GetBlock(s.chain.CurrentBlockHash(), false)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not get current block: %s", err), err)
	}
	ic, err := s.chain.GetTestVM(tx, fakeBlock, nil)
	if err != nil {
		if err != nil {
			return nil, response.NewInternalServerError(fmt.Sprintf("Could not create execute context: %s", err), err)
		}
	}
	var left uint64
	if inner.To() == nil {
		_, _, left, err = ic.VM.Create(ic, inner.Data(), TestGas, inner.Value())
	} else {
		_, left, err = ic.VM.Call(ic, *tx.To(), tx.Data(), TestGas, tx.Value())
	}
	if err != nil {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Could not executing data: %s", err), err)
	}
	ltx.Gas = TestGas - left
	err = acc.SignTx(s.chainId, tx)
	if err != nil {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Could not sign tx: %s", err), err)
	}
	raw, err := io.ToByteArray(tx)
	if err != nil {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Could not serialize tx: %s", err), err)
	}
	return hexutil.Encode(raw), nil
}

func (s *Server) eth_sendTransaction(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	data := []byte(param.RawMessage)
	txObj := result.TransactionObject{}
	err := json.Unmarshal(data, &txObj)
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	acc := s.getWalletAccount(txObj.From)
	if acc == nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not found accout to sign tx: %s", err), errors.New("account not found"))
	}
	ltx := &types.LegacyTx{
		Nonce:    s.chain.GetNonce(txObj.From),
		GasPrice: s.chain.GetGasPrice(),
		To:       txObj.To,
		Value:    txObj.Value,
		Data:     txObj.Data,
	}
	inner := &transaction.EthTx{
		Transaction: *types.NewTx(ltx),
		ChainID:     s.chainId,
		Sender:      txObj.From,
	}
	tx := transaction.NewTx(inner)
	fakeBlock, _, err := s.chain.GetBlock(s.chain.CurrentBlockHash(), false)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not get current block: %s", err), err)
	}
	ic, err := s.chain.GetTestVM(tx, fakeBlock, nil)
	if err != nil {
		if err != nil {
			return nil, response.NewInternalServerError(fmt.Sprintf("Could not create execute context: %s", err), err)
		}
	}
	var left uint64
	if inner.To() == nil {
		_, _, left, err = ic.VM.Create(ic, tx.Data(), TestGas, tx.Value())
	} else {
		_, left, err = ic.VM.Call(ic, *tx.To(), tx.Data(), TestGas, tx.Value())
	}
	if err != nil {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Could not executing data: %s", err), err)
	}
	ltx.Gas = TestGas - left
	netfee := transaction.CalculateNetworkFee(tx, s.chain.FeePerByte())
	ltx.Gas += netfee
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not calculate network fee: %s", err), err)
	}
	err = acc.SignTx(s.chainId, tx)
	if err != nil {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Could not sign tx: %s", err), err)
	}
	return getRelayResult(s.coreServer.RelayTxn(tx), tx.Hash())
}

func (s *Server) eth_sendRawTransaction(params request.Params) (interface{}, *response.Error) {
	if len(params) < 1 {
		return nil, response.NewInvalidParamsError("not enough parameters", nil)
	}
	rawTx, err := params[0].GetBytesHex()
	if err != nil {
		return nil, response.NewInvalidParamsError(fmt.Sprintf("invalid hex: %s", err), err)
	}
	etx, err := transaction.NewEthTxFromBytes(rawTx)
	if err != nil {
		return nil, response.NewInvalidParamsError("can't unmarshal raw transaction", err)
	}
	tx := transaction.NewTx(etx)
	return getRelayResult(s.coreServer.RelayTxn(tx), tx.Hash())
}

func (s *Server) eth_call(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	data := []byte(param.RawMessage)
	txObj := result.TransactionObject{}
	err := json.Unmarshal(data, &txObj)
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	ltx := &types.LegacyTx{
		Nonce:    s.chain.GetNonce(txObj.From) + 1,
		GasPrice: s.chain.GetGasPrice(),
		Gas:      txObj.Gas,
		To:       txObj.To,
		Value:    txObj.Value,
		Data:     txObj.Data,
	}
	inner := &transaction.EthTx{
		Transaction: *types.NewTx(ltx),
		ChainID:     s.chainId,
		Sender:      txObj.From,
	}
	tx := transaction.NewTx(inner)
	block, _, err := s.chain.GetBlock(s.chain.CurrentBlockHash(), false)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not get current block: %s", err), err)
	}
	ic, err := s.chain.GetTestVM(tx, block, nil)
	if err != nil {
		if err != nil {
			return nil, response.NewInternalServerError(fmt.Sprintf("Could not create execute context: %s", err), err)
		}
	}
	var ret []byte
	if inner.To() == nil {
		ret, _, _, err = ic.VM.Create(ic, tx.Data(), TestGas, tx.Value())
	} else {
		ret, _, err = ic.VM.Call(ic, *tx.To(), tx.Data(), TestGas, tx.Value())
	}
	if err != nil {
		return nil, response.NewRPCError(fmt.Sprintf("Could not executing data: %s", err), hexutil.Encode(ret), err)
	}
	return hexutil.Encode(ret), nil
}

func (s *Server) eth_estimateGas(reqParams request.Params) (interface{}, *response.Error) {
	param := reqParams.Value(0)
	data := []byte(param.RawMessage)
	txObj := result.TransactionObject{}
	err := json.Unmarshal(data, &txObj)
	if err != nil {
		return nil, response.NewInvalidParamsError(fmt.Sprintf("Could not unmarshal tx object: %s", err), err)
	}
	var tx *transaction.Transaction
	if txObj.Witness == nil {
		ltx := &types.LegacyTx{
			Nonce:    s.chain.GetNonce(txObj.From) + 1,
			GasPrice: s.chain.GetGasPrice(),
			Gas:      txObj.Gas,
			To:       txObj.To,
			Value:    txObj.Value,
			Data:     txObj.Data,
		}
		inner := &transaction.EthTx{
			Transaction: *types.NewTx(ltx),
			ChainID:     s.chainId,
			Sender:      txObj.From,
		}
		tx = transaction.NewTx(inner)
	} else {
		inner := &transaction.NeoTx{
			Nonce:    s.chain.GetNonce(txObj.From) + 1,
			From:     txObj.From,
			GasPrice: s.chain.GetGasPrice(),
			Gas:      txObj.Gas,
			To:       txObj.To,
			Value:    txObj.Value,
			Data:     txObj.Data,
			Witness:  *txObj.Witness,
		}
		if len(inner.Witness.VerificationScript) == 0 {
			return nil, response.NewInvalidParamsError("missing verification script", nil)
		}
		tx = transaction.NewTx(inner)
	}
	block, _, err := s.chain.GetBlock(s.chain.CurrentBlockHash(), false)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not get current block: %s", err), err)
	}
	fakeBlock := *block
	ic, err := s.chain.GetTestVM(tx, &fakeBlock, nil)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not create execute context: %s", err), err)
	}
	var (
		left uint64
		ret  []byte
		gas  uint64 = TestGas
	)
	if tx.Gas() != 0 {
		gas = tx.Gas()
	}
	if tx.To() == nil {
		ret, _, left, err = ic.VM.Create(ic, tx.Data(), gas, tx.Value())
	} else {
		ret, left, err = ic.VM.Call(ic, *tx.To(), tx.Data(), gas, tx.Value())
	}
	if err != nil {
		return nil, response.NewRPCError(fmt.Sprintf("Could not executing data: %s", err), hexutil.Encode(ret), err)
	}
	gas = gas - left
	balance := s.chain.GetUtilityTokenBalance(tx.From())
	gasPrice := s.chain.GetGasPrice()
	if tx.GasPrice().Cmp(gasPrice) > 0 {
		gasPrice = tx.GasPrice()
	}
	// execute again to ensure gas
	if tx.To() != nil {
		for i := 0; i < int(params.CallCreateDepth) && big.NewInt(0).Mul(gasPrice, big.NewInt(int64(gas))).Cmp(balance) <= 0; i++ {
			ic, err := s.chain.GetTestVM(tx, &fakeBlock, nil)
			if err != nil {
				return nil, response.NewInternalServerError(fmt.Sprintf("Could not create execute context: %s", err), err)
			}
			_, _, err = ic.VM.Call(ic, *tx.To(), tx.Data(), gas, tx.Value())
			if err == nil {
				s.log.Debug("estimate gas try", zap.Int("count", i))
				break
			}
			gas += uint64(float64(params.SstoreSentryGasEIP2200) * math.Pow(1.015625, float64(i)))
		}
		if err != nil {
			return nil, response.NewRPCError("Could not estimate gas", hexutil.EncodeUint64(gas), err)
		}
	}
	feePerByte := s.chain.GetFeePerByte()
	netfee := transaction.CalculateNetworkFee(tx, feePerByte)
	if err != nil {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Could not calculate network fee: %s", err), err)
	}
	gas += netfee
	return hexutil.EncodeUint64(gas), nil
}

func (s *Server) eth_getBlockByHash(params request.Params) (interface{}, *response.Error) {
	param0 := params.Value(0)
	hash, err := param0.GetHash()
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}
	full := true
	param1 := params.Value(1)
	if param1 != nil {
		f, err := param1.GetBoolean()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		full = f
	}
	return s.eth_getBlock(hash, full)
}

func (s *Server) eth_getBlockByNumber(params request.Params) (interface{}, *response.Error) {
	sh, err := params.Value(0).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	var hash common.Hash
	h, err := hexutil.DecodeUint64(sh)
	if err != nil {
		if sh == "latest" {
			hash = s.chain.CurrentBlockHash()
		} else if sh == "earliest" || sh == "pending" {
			return nil, response.NewInternalServerError(fmt.Sprintf("unsupport param %s", sh), errors.New("unsupport param"))
		} else {
			return nil, response.ErrInvalidParams
		}
	} else {
		hash = s.chain.GetHeaderHash(int(h))
	}
	full := true
	param1 := params.Value(1)
	if param1 != nil {
		full, err = param1.GetBoolean()
		if err != nil {
			return nil, response.NewInvalidParamsError(fmt.Sprintf("%s", err), err)
		}
	}
	return s.eth_getBlock(hash, full)
}

func (s *Server) eth_getBlock(hash common.Hash, full bool) (*result.Block, *response.Error) {
	block, receipt, err := s.chain.GetBlock(hash, true)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
	}
	sr, err := s.chain.GetStateModule().GetStateRoot(block.Index)
	if err != nil {
		return nil, response.NewInternalServerError("can't get state root", err)
	}
	validators, err := s.chain.GetValidators(block.Index)
	if err != nil {
		return nil, response.NewInternalServerError("can't get validators", err)
	}
	return result.NewBlock(s.chain, block, receipt, sr, validators[block.PrimaryIndex].Address(), full, s.chain.GetConfig()), nil
}

func (s *Server) eth_getTransactionByHash(params request.Params) (interface{}, *response.Error) {
	txHash, err := params.Value(0).GetHash()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	tx, receipt, err := s.chain.GetTransaction(txHash)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, response.NewInternalServerError(fmt.Sprintf("Failed to get tx: %s", err), err)
	}
	//pending
	if receipt == nil {
		return nil, nil
	}
	_header := s.chain.GetHeaderHash(int(receipt.BlockNumber.Uint64()))
	header, err := s.chain.GetHeader(_header)
	if err != nil {
		return nil, response.NewRPCError("Failed to get header for the transaction", err.Error(), err)
	}
	return result.NewTransactionOutputRaw(tx, header, receipt), nil
}

func (s *Server) eth_getTransactionByBlockHashAndIndex(params request.Params) (interface{}, *response.Error) {
	blockHash, err := params.Value(0).GetHash()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	index, err := params.Value(1).GetInt()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	block, _, err := s.chain.GetBlock(blockHash, false)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", blockHash), err)
	}
	if index < 0 || index >= len(block.Transactions) {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Index exceeds tx count in block: %d", index), errors.New("index exceeds tx count"))
	}
	txHash := block.Transactions[index].Hash()
	tx, receipt, err := s.chain.GetTransaction(txHash)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Failed to get tx: %s", err), err)
	}
	return result.NewTransactionOutputRaw(tx, &block.Header, receipt), nil
}

func (s *Server) eth_getTransactionByBlockNumberAndIndex(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	num, respErr := s.blockHeightFromParam(param)
	if respErr != nil {
		return nil, respErr
	}
	blockHash := s.chain.GetHeaderHash(num)
	index, err := params.Value(1).GetInt()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	block, _, err := s.chain.GetBlock(blockHash, true)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", blockHash), err)
	}
	if index < 0 || index >= len(block.Transactions) {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Index exceeds tx count in block: %d", index), errors.New("index exceeds tx count"))
	}
	txHash := block.Transactions[index].Hash()
	tx, receipt, err := s.chain.GetTransaction(txHash)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Failed to get tx: %s", err), err)
	}
	return result.NewTransactionOutputRaw(tx, &block.Header, receipt), nil
}

func (s *Server) eth_getTransactionReceipt(params request.Params) (interface{}, *response.Error) {
	txHash, err := params.Value(0).GetHash()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	tx, receipt, err := s.chain.GetTransaction(txHash)
	if err != nil || receipt == nil {
		return nil, nil
	}
	return result.NewRReceipt(receipt, tx), nil
}

func (s *Server) eth_newFilter(params request.Params) (interface{}, *response.Error) {
	return nil, response.NewInternalServerError("Umimplemented", errors.New("umimplemented"))
}

func (s *Server) eth_newBlockFilter(params request.Params) (interface{}, *response.Error) {
	return nil, response.NewInternalServerError("Umimplemented", errors.New("umimplemented"))
}

func (s *Server) eth_newPendingTransactionFilter(params request.Params) (interface{}, *response.Error) {
	return nil, response.NewInternalServerError("Umimplemented", errors.New("umimplemented"))
}

func (s *Server) eth_uninstallFilter(params request.Params) (interface{}, *response.Error) {
	return nil, response.NewInternalServerError("Umimplemented", errors.New("umimplemented"))
}

func (s *Server) eth_getFilterChanges(params request.Params) (interface{}, *response.Error) {
	return nil, response.NewInternalServerError("Umimplemented", errors.New("umimplemented"))
}

func (s *Server) eth_getFilterLogs(params request.Params) (interface{}, *response.Error) {
	return nil, response.NewInternalServerError("Umimplemented", errors.New("umimplemented"))
}

func (s *Server) eth_getLogs(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	data := []byte(param.RawMessage)
	filter := &filters.LogFilter{}
	err := json.Unmarshal(data, filter)
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	logs, err := s.chain.GetLogs(filter)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not get logs: %s", err), err)
	}
	return logs, nil
}

func (s *Server) eth_getUncleByBlockHashAndIndex(_ request.Params) (interface{}, *response.Error) {
	return nil, nil
}

func (s *Server) eth_feeHistory(_ request.Params) (interface{}, *response.Error) {
	_, receipt, err := s.chain.GetBlock(s.chain.CurrentBlockHash(), false)
	if err != nil {
		return nil, response.NewInternalServerError("can't get current block", err)
	}
	fh, err := result.NewFeeHistory([]*types.Receipt{receipt}, s.chain.GetGasPrice())
	if err != nil {
		return nil, response.NewInternalServerError("can't create fee history", err)
	}
	return fh, nil
}

// -- end eth api.

// -- start trace api

func (s *Server) trace_call(params request.Params) (interface{}, *response.Error) {
	param := params.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	data := []byte(param.RawMessage)
	txObj := result.TransactionObject{}
	err := json.Unmarshal(data, &txObj)
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	ltx := &types.LegacyTx{
		Nonce:    s.chain.GetNonce(txObj.From) + 1,
		GasPrice: s.chain.GetGasPrice(),
		Gas:      txObj.Gas,
		To:       txObj.To,
		Value:    txObj.Value,
		Data:     txObj.Data,
	}
	inner := &transaction.EthTx{
		Transaction: *types.NewTx(ltx),
		ChainID:     s.chainId,
		Sender:      txObj.From,
	}
	tx := transaction.NewTx(inner)
	block, _, err := s.chain.GetBlock(s.chain.CurrentBlockHash(), false)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not get current block: %s", err), err)
	}
	tracer := response.NewVMLogger()
	ic, err := s.chain.GetTestVM(tx, block, tracer)
	if err != nil {
		if err != nil {
			return nil, response.NewInternalServerError(fmt.Sprintf("Could not create execute context: %s", err), err)
		}
	}
	var ret []byte
	if inner.To() == nil {
		ret, _, _, err = ic.VM.Create(ic, tx.Data(), TestGas, tx.Value())
	} else {
		ret, _, err = ic.VM.Call(ic, *tx.To(), tx.Data(), TestGas, tx.Value())
	}
	res := result.TraceResult{
		Trace: tracer.Result(),
		Ret:   hexutil.Bytes(ret),
	}
	if err != nil {
		res.Err = err.Error()
	}
	return res, nil
}

// -- end trace api

// -- start gether api

func (s *Server) txpool_content(_ request.Params) (interface{}, *response.Error) {
	mempool := s.chain.GetMemPool()
	return result.NewTxPool(mempool.GetVerifiedTransactions()), nil
}

// -- end gether api

func (s *Server) getBestBlockHash(_ request.Params) (interface{}, *response.Error) {
	return s.chain.CurrentBlockHash().String(), nil
}

func (s *Server) getBlock(params request.Params) (interface{}, *response.Error) {
	if len(params) < 1 {
		return nil, response.ErrInvalidParams
	}
	var hash common.Hash
	height, err := s.blockHeightFromParam(&params[0])
	if err != nil {
		hash, err = s.blockHashFromParam(&params[0])
		if err != nil {
			return nil, response.NewInvalidParamsError("can't parse height or hash", err)
		}
	} else {
		hash = s.chain.GetHeaderHash(height)
	}
	raw := false
	if len(params) > 1 {
		r, e := params[1].GetBoolean()
		if err != nil {
			return nil, response.NewInvalidParamsError("can't parse bool", e)
		}
		raw = r
	}
	block, _, e := s.chain.GetBlock(hash, true)
	if e != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
	}
	if raw {
		b, e := io.ToByteArray(block)
		if e != nil {
			return nil, response.NewInternalServerError("can't encode block", e)
		}
		return hex.EncodeToString(b), nil
	} else {
		return block, nil
	}
}

func (s *Server) getBlockCount(_ request.Params) (interface{}, *response.Error) {
	return s.chain.BlockHeight() + 1, nil
}

func (s *Server) getBlockHeaderCount(_ request.Params) (interface{}, *response.Error) {
	return s.chain.HeaderHeight() + 1, nil
}

func (s *Server) getConnectionCount(_ request.Params) (interface{}, *response.Error) {
	return s.coreServer.PeerCount(), nil
}

func (s *Server) blockHashFromParam(param *request.Param) (common.Hash, *response.Error) {
	var (
		hash common.Hash
		err  error
	)
	if param == nil {
		return hash, response.ErrInvalidParams
	}

	if hash, err = param.GetHash(); err != nil {
		num, respErr := s.blockHeightFromParam(param)
		if respErr != nil {
			return hash, respErr
		}
		hash = s.chain.GetHeaderHash(num)
	}
	return hash, nil
}

func (s *Server) getBlockHash(reqParams request.Params) (interface{}, *response.Error) {
	num, err := s.blockHeightFromParam(reqParams.Value(0))
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	return s.chain.GetHeaderHash(num), nil
}

func (s *Server) getVersion(_ request.Params) (interface{}, *response.Error) {
	port, err := s.coreServer.Port()
	if err != nil {
		return nil, response.NewInternalServerError("Cannot fetch tcp port", err)
	}
	validators, err := s.chain.GetCurrentValidators()
	if err != nil {
		return nil, response.NewInternalServerError("failed get current validators", err)
	}
	cfg := s.chain.GetConfig()
	return result.Version{
		ChainID:   s.chainId,
		TCPPort:   port,
		Nonce:     s.coreServer.ID(),
		UserAgent: s.coreServer.UserAgent,
		Protocol: result.Protocol{
			ChainID:                   cfg.ChainID,
			MillisecondsPerBlock:      cfg.SecondsPerBlock * 1000,
			MaxTraceableBlocks:        cfg.MaxTraceableBlocks,
			MaxTransactionsPerBlock:   cfg.MaxTransactionsPerBlock,
			MemoryPoolMaxTransactions: cfg.MemPoolSize,
			ValidatorsCount:           byte(len(validators)),
			InitialGasDistribution:    cfg.InitialGASSupply,
		},
	}, nil
}

func (s *Server) getPeers(_ request.Params) (interface{}, *response.Error) {
	peers := result.NewGetPeers()
	peers.AddUnconnected(s.coreServer.UnconnectedPeers())
	peers.AddConnected(s.coreServer.ConnectedPeers())
	peers.AddBad(s.coreServer.BadPeers())
	return peers, nil
}

func (s *Server) getRawMempool(reqParams request.Params) (interface{}, *response.Error) {
	verbose, _ := reqParams.Value(0).GetBoolean()
	mp := s.chain.GetMemPool()
	hashList := make([]common.Hash, 0)
	for _, item := range mp.GetVerifiedTransactions() {
		hashList = append(hashList, item.Hash())
	}
	if !verbose {
		return hashList, nil
	}
	return result.RawMempool{
		Height:     s.chain.BlockHeight(),
		Verified:   hashList,
		Unverified: []common.Hash{}, // avoid `null` result
	}, nil
}

func (s *Server) validateAddress(reqParams request.Params) (interface{}, *response.Error) {
	param, err := reqParams.Value(0).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	return result.ValidateAddress{
		Address: reqParams.Value(0),
		IsValid: common.IsHexAddress(param),
	}, nil
}

// calculateNetworkFee calculates network fee for the transaction.
func (s *Server) calculateGas(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 1 {
		return 0, response.ErrInvalidParams
	}
	byteTx, err := reqParams[0].GetBytesHex()
	if err != nil {
		return 0, response.WrapErrorWithData(response.ErrInvalidParams, err)
	}
	neoTx, err := transaction.NewNeoTxFromBytes(byteTx)
	if err != nil {
		return 0, response.WrapErrorWithData(response.ErrInvalidParams, err)
	}
	if len(neoTx.Witness.VerificationScript) == 0 {
		return nil, response.NewInvalidParamsError("missing verification script", nil)
	}
	tx := transaction.NewTx(neoTx)
	feePerByte := s.chain.GetFeePerByte()
	netfee := transaction.CalculateNetworkFee(tx, feePerByte)
	if err != nil {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Could not calculate network fee: %s", err), err)
	}
	block, _, err := s.chain.GetBlock(s.chain.CurrentBlockHash(), false)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Could not get current block: %s", err), err)
	}
	fakeBlock := *block
	ic, err := s.chain.GetTestVM(tx, &fakeBlock, nil)
	if err != nil {
		if err != nil {
			return nil, response.NewInternalServerError(fmt.Sprintf("Could not create execute context: %s", err), err)
		}
	}
	var left uint64
	if neoTx.To == nil {
		_, _, left, err = ic.VM.Create(ic, tx.Data(), TestGas, tx.Value())
	} else {
		_, left, err = ic.VM.Call(ic, *tx.To(), tx.Data(), TestGas, tx.Value())
	}
	if err != nil {
		return nil, response.NewInvalidRequestError(fmt.Sprintf("Could not executing data: %s", err), err)
	}
	neoTx.Gas = TestGas - left
	neoTx.Gas += netfee + params.SstoreSentryGasEIP2200
	if err != nil {
		return 0, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("failed to compute tx size: %w", err))
	}
	return result.NetworkFee{Value: neoTx.Gas}, nil
}

// getContractScriptHashFromParam returns the contract script hash by hex contract hash, address, id or native contract name.
func (s *Server) contractScriptHashFromParam(param *request.Param) (common.Address, *response.Error) {
	var result common.Address
	if param == nil {
		return result, response.ErrInvalidParams
	}
	nameOrHashOrIndex, err := param.GetString()
	if err != nil {
		return result, response.ErrInvalidParams
	}
	result, err = param.GetAddressFromHex()
	if err == nil {
		return result, nil
	}
	result, err = s.chain.GetNativeContractScriptHash(nameOrHashOrIndex)
	if err == nil {
		return result, nil
	}
	id, err := strconv.Atoi(nameOrHashOrIndex)
	if err != nil {
		return result, response.NewRPCError("Unknown contract", "", err)
	}
	if err := checkInt32(id); err != nil {
		return result, response.WrapErrorWithData(response.ErrInvalidParams, err)
	}
	return result, nil
}

func makeStorageKey(hash common.Address, key []byte) []byte {
	skey := make([]byte, 20+len(key))
	copy(skey, hash.Bytes())
	copy(skey[20:], key)
	return skey
}

var errKeepOnlyLatestState = errors.New("'KeepOnlyLatestState' setting is enabled")

func (s *Server) getProof(ps request.Params) (interface{}, *response.Error) {
	if s.chain.GetConfig().KeepOnlyLatestState {
		return nil, response.NewInvalidRequestError("'getproof' is not supported", errKeepOnlyLatestState)
	}
	root, err := ps.Value(0).GetHash()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	sc, err := ps.Value(1).GetAddressFromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	key, err := ps.Value(2).GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	cs, respErr := s.getHistoricalContractState(root, sc)
	if respErr != nil {
		return nil, respErr
	}
	skey := makeStorageKey(cs.Address, key)
	proof, err := s.chain.GetStateModule().GetStateProof(root, skey)
	if err != nil {
		return nil, response.NewInternalServerError("failed to get proof", err)
	}
	return &result.ProofWithKey{
		Key:   skey,
		Proof: proof,
	}, nil
}

func (s *Server) verifyProof(ps request.Params) (interface{}, *response.Error) {
	if s.chain.GetConfig().KeepOnlyLatestState {
		return nil, response.NewInvalidRequestError("'verifyproof' is not supported", errKeepOnlyLatestState)
	}
	root, err := ps.Value(0).GetHash()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	proofStr, err := ps.Value(1).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	var p result.ProofWithKey
	if err := p.FromString(proofStr); err != nil {
		return nil, response.ErrInvalidParams
	}
	vp := new(result.VerifyProof)
	val, ok := mpt.VerifyProof(root, p.Key, p.Proof)
	if ok {
		vp.Value = val
	}
	return vp, nil
}

func (s *Server) getState(ps request.Params) (interface{}, *response.Error) {
	root, err := ps.Value(0).GetHash()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("invalid stateroot"))
	}
	if s.chain.GetConfig().KeepOnlyLatestState {
		curr, err := s.chain.GetStateModule().GetStateRoot(s.chain.BlockHeight())
		if err != nil {
			return nil, response.NewInternalServerError("failed to get current stateroot", err)
		}
		if curr.Root != root {
			return nil, response.NewInvalidRequestError("'getstate' is not supported for old states", errKeepOnlyLatestState)
		}
	}
	csHash, err := ps.Value(1).GetAddressFromHex()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("invalid contract hash"))
	}
	key, err := ps.Value(2).GetBytesHex()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("invalid key"))
	}
	cs, respErr := s.getHistoricalContractState(root, csHash)
	if respErr != nil {
		return nil, respErr
	}
	sKey := makeStorageKey(cs.Address, key)
	res, err := s.chain.GetStateModule().GetState(root, sKey)
	if err != nil {
		return nil, response.NewInternalServerError("failed to get historical item state", err)
	}
	return res, nil
}

func (s *Server) findStates(ps request.Params) (interface{}, *response.Error) {
	root, err := ps.Value(0).GetHash()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("invalid stateroot"))
	}
	if s.chain.GetConfig().KeepOnlyLatestState {
		curr, err := s.chain.GetStateModule().GetStateRoot(s.chain.BlockHeight())
		if err != nil {
			return nil, response.NewInternalServerError("failed to get current stateroot", err)
		}
		if curr.Root != root {
			return nil, response.NewInvalidRequestError("'findstates' is not supported for old states", errKeepOnlyLatestState)
		}
	}
	csHash, err := ps.Value(1).GetAddressFromHex()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("invalid contract hash: %w", err))
	}
	prefix, err := ps.Value(2).GetBytesHex()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("invalid prefix: %w", err))
	}
	var (
		key   []byte
		count = s.config.MaxFindResultItems
	)
	if len(ps) > 3 {
		key, err = ps.Value(3).GetBytesHex()
		if err != nil {
			return nil, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("invalid key: %w", err))
		}
		if len(key) > 0 {
			if !bytes.HasPrefix(key, prefix) {
				return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("key doesn't match prefix"))
			}
			key = key[len(prefix):]
		} else {
			// empty ("") key shouldn't exclude item matching prefix from the result
			key = nil
		}
	}
	if len(ps) > 4 {
		count, err = ps.Value(4).GetInt()
		if err != nil {
			return nil, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("invalid count: %w", err))
		}
		if count > s.config.MaxFindResultItems {
			count = s.config.MaxFindResultItems
		}
	}
	cs, respErr := s.getHistoricalContractState(root, csHash)
	if respErr != nil {
		return nil, respErr
	}
	pKey := makeStorageKey(cs.Address, prefix)
	kvs, err := s.chain.GetStateModule().FindStates(root, pKey, key, count+1) // +1 to define result truncation
	if err != nil {
		return nil, response.NewInternalServerError("failed to find historical items", err)
	}
	res := result.FindStates{}
	if len(kvs) == count+1 {
		res.Truncated = true
		kvs = kvs[:len(kvs)-1]
	}
	if len(kvs) > 0 {
		proof, err := s.chain.GetStateModule().GetStateProof(root, kvs[0].Key)
		if err != nil {
			return nil, response.NewInternalServerError("failed to get first proof", err)
		}
		res.FirstProof = &result.ProofWithKey{
			Key:   kvs[0].Key,
			Proof: proof,
		}
	}
	if len(kvs) > 1 {
		proof, err := s.chain.GetStateModule().GetStateProof(root, kvs[len(kvs)-1].Key)
		if err != nil {
			return nil, response.NewInternalServerError("failed to get first proof", err)
		}
		res.LastProof = &result.ProofWithKey{
			Key:   kvs[len(kvs)-1].Key,
			Proof: proof,
		}
	}
	res.Results = make([]result.KeyValue, len(kvs))
	for i, kv := range kvs {
		res.Results[i] = result.KeyValue{
			Key:   kv.Key[4:], // cut contract ID as it is done in C#
			Value: kv.Value,
		}
	}
	return res, nil
}

func (s *Server) getHistoricalContractState(root common.Hash, csHash common.Address) (*state.Contract, *response.Error) {
	csKey := makeStorageKey(native.ManagementAddress, native.MakeContractKey(csHash))
	csBytes, err := s.chain.GetStateModule().GetState(root, csKey)
	if err != nil {
		return nil, response.NewInternalServerError("failed to get historical contract state", err)
	}
	contract := new(state.Contract)
	err = io.FromByteArray(contract, csBytes)
	return contract, response.NewRPCError("Failed get contract state", "", err)
}

func (s *Server) getStateHeight(_ request.Params) (interface{}, *response.Error) {
	var height = s.chain.BlockHeight()
	var stateHeight = s.chain.GetStateModule().CurrentValidatedHeight()
	return &result.StateHeight{
		Local:     height,
		Validated: stateHeight,
	}, nil
}

func (s *Server) getStateRoot(ps request.Params) (interface{}, *response.Error) {
	p := ps.Value(0)
	if p == nil {
		return nil, response.NewRPCError("Invalid parameter.", "", nil)
	}
	var rt *state.MPTRoot
	var h common.Hash
	height, err := p.GetIntStrict()
	if err == nil {
		if err := checkUint32(height); err != nil {
			return nil, response.WrapErrorWithData(response.ErrInvalidParams, err)
		}
		rt, err = s.chain.GetStateModule().GetStateRoot(uint32(height))
	} else if h, err = p.GetHash(); err == nil {
		var hdr *block.Header
		hdr, err = s.chain.GetHeader(h)
		if err == nil {
			rt, err = s.chain.GetStateModule().GetStateRoot(hdr.Index)
		}
	}
	if err != nil {
		return nil, response.NewRPCError("Unknown state root.", "", err)
	}
	return rt, nil
}

func (s *Server) getStorage(ps request.Params) (interface{}, *response.Error) {
	hash, rErr := s.contractScriptHashFromParam(ps.Value(0))
	if rErr == response.ErrUnknown {
		return nil, nil
	}
	if rErr != nil {
		return nil, rErr
	}

	key, err := ps.Value(1).GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	item := s.chain.GetStorageItem(hash, key)
	if item == nil {
		return "", nil
	}

	return []byte(item), nil
}

func (s *Server) getrawtransaction(reqParams request.Params) (interface{}, *response.Error) {
	txHash, err := reqParams.Value(0).GetHash()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	tx, receipt, err := s.chain.GetTransaction(txHash)
	if err != nil {
		err = fmt.Errorf("invalid transaction %s: %w", txHash, err)
		return nil, response.NewRPCError("Unknown transaction", err.Error(), err)
	}
	if v, _ := reqParams.Value(1).GetBoolean(); v {
		if receipt == nil {
			return result.NewTransactionOutputRaw(tx, nil, nil), nil
		}
		_header := s.chain.GetHeaderHash(int(receipt.BlockNumber.Uint64()))
		header, err := s.chain.GetHeader(_header)
		if err != nil {
			return nil, response.NewRPCError("Failed to get block header for the transaction", err.Error(), err)
		}
		return result.NewTransactionOutputRaw(tx, header, receipt), nil
	}
	b, err := tx.Bytes()
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("failed encode tx: %s", err), err)
	}
	return hexutil.Bytes(b), nil
}

func (s *Server) getTransactionHeight(ps request.Params) (interface{}, *response.Error) {
	h, err := ps.Value(0).GetHash()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	_, r, err := s.chain.GetTransaction(h)
	if err != nil || r == nil {
		return nil, response.NewRPCError("Unknown transaction", "", nil)
	}

	return r.BlockNumber.Uint64(), nil
}

// getContractState returns contract state (contract information, according to the contract script hash,
// contract id or native contract name).
func (s *Server) getContractState(reqParams request.Params) (interface{}, *response.Error) {
	scriptHash, err := s.contractScriptHashFromParam(reqParams.Value(0))
	if err != nil {
		return nil, err
	}
	cs := s.chain.GetContractState(scriptHash)
	if cs == nil {
		return nil, response.NewRPCError("Unknown contract", "", nil)
	}
	return cs, nil
}

func (s *Server) getFeePerByte(_ request.Params) (interface{}, *response.Error) {
	return s.chain.GetFeePerByte(), nil
}

func (s *Server) getNativeContracts(_ request.Params) (interface{}, *response.Error) {
	return s.chain.GetNatives(), nil
}

// getBlockSysFee returns the system fees of the block, based on the specified index.
func (s *Server) getBlockGas(reqParams request.Params) (interface{}, *response.Error) {
	num, err := s.blockHeightFromParam(reqParams.Value(0))
	if err != nil {
		return 0, response.NewRPCError("Invalid height", "", nil)
	}

	headerHash := s.chain.GetHeaderHash(num)
	block, _, errBlock := s.chain.GetBlock(headerHash, true)
	if errBlock != nil {
		return 0, response.NewRPCError(errBlock.Error(), "", nil)
	}

	var blockGas uint64
	for _, tx := range block.Transactions {
		blockGas += tx.Gas()
	}

	return blockGas, nil
}

// getBlockHeader returns the corresponding block header information according to the specified script hash.
func (s *Server) getBlockHeader(reqParams request.Params) (interface{}, *response.Error) {
	param := reqParams.Value(0)
	hash, respErr := s.blockHashFromParam(param)
	if respErr != nil {
		index, err := s.blockHeightFromParam(param)
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		hash = s.chain.GetHeaderHash(index)
		if hash == (common.Hash{}) {
			return nil, response.NewRPCError("unknown block", "", nil)
		}
	}
	verbose, _ := reqParams.Value(1).GetBoolean()
	h, err := s.chain.GetHeader(hash)
	if err != nil {
		return nil, response.NewRPCError("unknown block", "", nil)
	}

	if verbose {
		return result.NewHeader(h, s.chain), nil
	}

	buf := io.NewBufBinWriter()
	h.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, response.NewInternalServerError("encoding error", buf.Err)
	}
	return buf.Bytes(), nil
}

func (s *Server) getNextValidators(_ request.Params) (interface{}, *response.Error) {
	validators, err := s.chain.GetCurrentValidators()
	if err != nil {
		return nil, response.NewInternalServerError("Failed to get validators", err)
	}
	return validators, nil
}

func (s *Server) getValidators(params request.Params) (interface{}, *response.Error) {
	p := params.Value(0)
	if p == nil {
		return nil, response.ErrInvalidParams
	}
	index, err := p.GetIntStrict()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	validators, err := s.chain.GetValidators(uint32(index))
	if err != nil {
		return nil, response.NewInternalServerError("Failed to get validators", err)
	}
	return validators, nil
}

func (s *Server) getConsensusAddress(_ request.Params) (interface{}, *response.Error) {
	addr, err := s.chain.GetConsensusAddress()
	if err != nil {
		return nil, response.NewInternalServerError("can't get committee members", err)
	}
	return addr, nil
}

func (s *Server) isBlocked(reqParams request.Params) (interface{}, *response.Error) {
	para1 := reqParams.Value(0)
	if para1 == nil {
		return nil, response.ErrInvalidParams
	}
	addr, err := para1.GetAddressFromHex()
	if err != nil {
		return nil, response.NewInternalServerError(err.Error(), err)
	}
	r := s.chain.IsBlocked(addr)
	return r, nil
}

// getRelayResult returns successful relay result or an error.
func getRelayResult(err error, hash common.Hash) (interface{}, *response.Error) {
	switch {
	case err == nil:
		return hash, nil
	case errors.Is(err, core.ErrAlreadyExists):
		return nil, response.WrapErrorWithData(response.ErrAlreadyExists, err)
	case errors.Is(err, core.ErrOOM):
		return nil, response.WrapErrorWithData(response.ErrOutOfMemory, err)
	case errors.Is(err, core.ErrPolicy):
		return nil, response.WrapErrorWithData(response.ErrPolicyFail, err)
	default:
		return nil, response.WrapErrorWithData(response.ErrValidationFailed, err)
	}
}

func (s *Server) sendrawtransaction(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 1 {
		return nil, response.NewInvalidParamsError("not enough parameters", nil)
	}
	byteTx, err := reqParams[0].GetBytesHex()
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}
	NeoTx, err := transaction.NewNeoTxFromBytes(byteTx)
	if err != nil {
		return nil, response.NewInvalidParamsError("can't decode transaction", err)
	}
	tx := transaction.NewTx(NeoTx)
	return getRelayResult(s.coreServer.RelayTxn(tx), tx.Hash())
}

func (s *Server) getMinted(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 1 {
		return nil, response.NewInvalidParamsError("not enough parameters", nil)
	}
	id, err := reqParams[0].GetInt()
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}
	txid, err := s.chain.GetMinted(int64(id))
	if err != nil {
		return nil, response.NewInternalServerError("can't get minted", err)
	}
	if txid == (common.Hash{}) {
		return nil, nil
	}
	return txid, nil
}

// subscribe handles subscription requests from websocket clients.
func (s *Server) subscribe(reqParams request.Params, sub *subscriber) (interface{}, *response.Error) {
	streamName, err := reqParams.Value(0).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	event, err := response.GetEventIDFromString(streamName)
	if err != nil || event == response.MissedEventID {
		return nil, response.ErrInvalidParams
	}
	// Optional filter.
	var filter interface{}
	if p := reqParams.Value(1); p != nil {
		param := *p
		jd := json.NewDecoder(bytes.NewReader(param.RawMessage))
		jd.DisallowUnknownFields()
		switch event {
		case response.BlockEventID:
			flt := new(request.BlockFilter)
			err = jd.Decode(flt)
			filter = *flt
		case response.TransactionEventID:
			flt := new(request.TxFilter)
			err = jd.Decode(flt)
			filter = *flt
		case response.NotificationEventID:
			flt := new(request.NotificationFilter)
			err = jd.Decode(flt)
			filter = *flt
		case response.ExecutionEventID:
			flt := new(request.ExecutionFilter)
			err = jd.Decode(flt)
			if err == nil && (flt.State == 1 || flt.State == 0) {
				filter = *flt
			} else if err == nil {
				err = errors.New("invalid state")
			}
		}
		if err != nil {
			return nil, response.ErrInvalidParams
		}
	}

	s.subsLock.Lock()
	defer s.subsLock.Unlock()
	select {
	case <-s.shutdown:
		return nil, response.NewInternalServerError("server is shutting down", nil)
	default:
	}
	var id int
	for ; id < len(sub.feeds); id++ {
		if sub.feeds[id].event == response.InvalidEventID {
			break
		}
	}
	if id == len(sub.feeds) {
		return nil, response.NewInternalServerError("maximum number of subscriptions is reached", nil)
	}
	sub.feeds[id].event = event
	sub.feeds[id].filter = filter
	s.subscribeToChannel(event)
	return strconv.FormatInt(int64(id), 10), nil
}

// subscribeToChannel subscribes RPC server to appropriate chain events if
// it's not yet subscribed for them. It's supposed to be called with s.subsLock
// taken by the caller.
func (s *Server) subscribeToChannel(event response.EventID) {
	switch event {
	case response.BlockEventID:
		if s.blockSubs == 0 {
			s.chain.SubscribeForBlocks(s.blockCh)
		}
		s.blockSubs++
	case response.TransactionEventID:
		if s.transactionSubs == 0 {
			s.chain.SubscribeForTransactions(s.transactionCh)
		}
		s.transactionSubs++
	case response.NotificationEventID:
		if s.notificationSubs == 0 {
			s.chain.SubscribeForNotifications(s.notificationCh)
		}
		s.notificationSubs++
	case response.ExecutionEventID:
		if s.executionSubs == 0 {
			s.chain.SubscribeForExecutions(s.executionCh)
		}
		s.executionSubs++
	}
}

// unsubscribe handles unsubscription requests from websocket clients.
func (s *Server) unsubscribe(reqParams request.Params, sub *subscriber) (interface{}, *response.Error) {
	id, err := reqParams.Value(0).GetInt()
	if err != nil || id < 0 {
		return nil, response.ErrInvalidParams
	}
	s.subsLock.Lock()
	defer s.subsLock.Unlock()
	if len(sub.feeds) <= id || sub.feeds[id].event == response.InvalidEventID {
		return nil, response.ErrInvalidParams
	}
	event := sub.feeds[id].event
	sub.feeds[id].event = response.InvalidEventID
	sub.feeds[id].filter = nil
	s.unsubscribeFromChannel(event)
	return true, nil
}

// unsubscribeFromChannel unsubscribes RPC server from appropriate chain events
// if there are no other subscribers for it. It's supposed to be called with
// s.subsLock taken by the caller.
func (s *Server) unsubscribeFromChannel(event response.EventID) {
	switch event {
	case response.BlockEventID:
		s.blockSubs--
		if s.blockSubs == 0 {
			s.chain.UnsubscribeFromBlocks(s.blockCh)
		}
	case response.TransactionEventID:
		s.transactionSubs--
		if s.transactionSubs == 0 {
			s.chain.UnsubscribeFromTransactions(s.transactionCh)
		}
	case response.NotificationEventID:
		s.notificationSubs--
		if s.notificationSubs == 0 {
			s.chain.UnsubscribeFromNotifications(s.notificationCh)
		}
	case response.ExecutionEventID:
		s.executionSubs--
		if s.executionSubs == 0 {
			s.chain.UnsubscribeFromExecutions(s.executionCh)
		}
	}
}

func (s *Server) handleSubEvents() {
	b, err := json.Marshal(response.Notification{
		JSONRPC: request.JSONRPCVersion,
		Event:   response.MissedEventID,
		Payload: make([]interface{}, 0),
	})
	if err != nil {
		s.log.Error("fatal: failed to marshal overflow event", zap.Error(err))
		return
	}
	overflowMsg, err := websocket.NewPreparedMessage(websocket.TextMessage, b)
	if err != nil {
		s.log.Error("fatal: failed to prepare overflow message", zap.Error(err))
		return
	}
chloop:
	for {
		var resp = response.Notification{
			JSONRPC: request.JSONRPCVersion,
			Payload: make([]interface{}, 1),
		}
		var msg *websocket.PreparedMessage
		select {
		case <-s.shutdown:
			break chloop
		case b := <-s.blockCh:
			resp.Event = response.BlockEventID
			resp.Payload[0] = b
		case execution := <-s.executionCh:
			resp.Event = response.ExecutionEventID
			resp.Payload[0] = execution
		case notification := <-s.notificationCh:
			resp.Event = response.NotificationEventID
			resp.Payload[0] = notification
		case tx := <-s.transactionCh:
			resp.Event = response.TransactionEventID
			resp.Payload[0] = tx
		}
		s.subsLock.RLock()
	subloop:
		for sub := range s.subscribers {
			if sub.overflown.Load() {
				continue
			}
			for i := range sub.feeds {
				if sub.feeds[i].Matches(&resp) {
					if msg == nil {
						b, err = json.Marshal(resp)
						if err != nil {
							s.log.Error("failed to marshal notification",
								zap.Error(err),
								zap.String("type", resp.Event.String()))
							break subloop
						}
						msg, err = websocket.NewPreparedMessage(websocket.TextMessage, b)
						if err != nil {
							s.log.Error("failed to prepare notification message",
								zap.Error(err),
								zap.String("type", resp.Event.String()))
							break subloop
						}
					}
					select {
					case sub.writer <- msg:
					default:
						sub.overflown.Store(true)
						// MissedEvent is to be delivered eventually.
						go func(sub *subscriber) {
							sub.writer <- overflowMsg
							sub.overflown.Store(false)
						}(sub)
					}
					// The message is sent only once per subscriber.
					break
				}
			}
		}
		s.subsLock.RUnlock()
	}
	// It's important to do it with lock held because no subscription routine
	// should be running concurrently to this one. And even if one is to run
	// after unlock, it'll see closed s.shutdown and won't subscribe.
	s.subsLock.Lock()
	// There might be no subscription in reality, but it's not a problem as
	// core.Blockchain allows unsubscribing non-subscribed channels.
	s.chain.UnsubscribeFromBlocks(s.blockCh)
	s.chain.UnsubscribeFromTransactions(s.transactionCh)
	s.chain.UnsubscribeFromNotifications(s.notificationCh)
	s.chain.UnsubscribeFromExecutions(s.executionCh)
	s.subsLock.Unlock()
drainloop:
	for {
		select {
		case <-s.blockCh:
		case <-s.executionCh:
		case <-s.notificationCh:
		case <-s.transactionCh:
		default:
			break drainloop
		}
	}
	// It's not required closing these, but since they're drained already
	// this is safe and it also allows to give a signal to Shutdown routine.
	close(s.blockCh)
	close(s.transactionCh)
	close(s.notificationCh)
	close(s.executionCh)
}

func (s *Server) blockHeightFromParam(param *request.Param) (int, *response.Error) {
	num, err := param.GetInt()
	if err != nil {
		return 0, response.ErrInvalidParams
	}

	if num < 0 || num > int(s.chain.BlockHeight()) {
		return 0, invalidBlockHeightError(0, num)
	}
	return num, nil
}

func (s *Server) packResponse(r *request.In, result interface{}, respErr *response.Error) response.Abstract {
	resp := response.Abstract{
		HeaderAndError: response.HeaderAndError{
			Header: response.Header{
				JSONRPC: r.JSONRPC,
				ID:      r.RawID,
			},
		},
	}
	if respErr != nil {
		resp.Error = respErr
	} else {
		resp.Result = result
	}
	return resp
}

// logRequestError is a request error logger.
func (s *Server) logRequestError(r *request.Request, jsonErr *response.Error) {
	logFields := []zap.Field{
		zap.Error(jsonErr.Cause),
	}

	if r.In != nil {
		logFields = append(logFields, zap.String("method", r.In.Method))
		params := request.Params(r.In.RawParams)
		logFields = append(logFields, zap.Any("params", params))
	}

	s.log.Error("Error encountered with rpc request", logFields...)
}

// writeHTTPErrorResponse writes an error response to the ResponseWriter.
func (s *Server) writeHTTPErrorResponse(r *request.In, w http.ResponseWriter, jsonErr *response.Error) {
	resp := s.packResponse(r, nil, jsonErr)
	s.writeHTTPServerResponse(&request.Request{In: r}, w, resp)
}

func (s *Server) writeHTTPServerResponse(r *request.Request, w http.ResponseWriter, resp response.AbstractResult) {
	// Errors can happen in many places and we can only catch ALL of them here.
	resp.RunForErrors(func(jsonErr *response.Error) {
		s.logRequestError(r, jsonErr)
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.In != nil {
		resp := resp.(response.Abstract)
		if resp.Error != nil {
			w.WriteHeader(resp.Error.HTTPCode)
		}
	}
	if s.config.EnableCORSWorkaround {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
	}

	encoder := json.NewEncoder(w)
	err := encoder.Encode(resp)

	if err != nil {
		switch {
		case r.In != nil:
			s.log.Error("Error encountered while encoding response",
				zap.String("err", err.Error()),
				zap.String("method", r.In.Method))
		case r.Batch != nil:
			s.log.Error("Error encountered while encoding batch response",
				zap.String("err", err.Error()))
		}
	}
}
