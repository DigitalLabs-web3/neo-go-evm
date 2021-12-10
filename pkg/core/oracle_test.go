package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	gio "io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	oracleModulePath           = filepath.Join("..", "services", "oracle")
	oracleContractNEFPath      = filepath.Join("test_data", "oracle_contract", "oracle.nef")
	oracleContractManifestPath = filepath.Join("test_data", "oracle_contract", "oracle.manifest.json")
)

// TestGenerateOracleContract generates helper contract that is able to call
// native Oracle contract and has callback method. It uses test chain to define
// Oracle and StdLib native hashes and saves generated NEF and manifest to ... folder.
// Set `saveState` flag to true and run the test to rewrite NEF and manifest files.
func TestGenerateOracleContract(t *testing.T) {
	const saveState = false

	bc := newTestChain(t)
	oracleHash := bc.contracts.Oracle.Hash
	stdHash := bc.contracts.Std.Hash

	w := io.NewBufBinWriter()
	emit.Int(w.BinWriter, 5)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.Int(w.BinWriter, int64(callflag.All))
	emit.String(w.BinWriter, "request")
	emit.Bytes(w.BinWriter, oracleHash.BytesBE())
	emit.Syscall(w.BinWriter, interopnames.SystemContractCall)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	emit.Opcodes(w.BinWriter, opcode.RET)

	// `handle` method aborts if len(userData) == 2 and does NOT perform witness checks
	// for the sake of contract code simplicity (the contract is used in multiple testchains).
	offset := w.Len()

	emit.Opcodes(w.BinWriter, opcode.OVER)
	emit.Opcodes(w.BinWriter, opcode.SIZE)
	emit.Int(w.BinWriter, 2)
	emit.Instruction(w.BinWriter, opcode.JMPNE, []byte{3})
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	emit.Int(w.BinWriter, 4) // url, userData, code, result
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.Int(w.BinWriter, 1)                                            // 1 byte (args count for `serialize`)
	emit.Opcodes(w.BinWriter, opcode.PACK)                              // 1 byte (pack args into array for `serialize`)
	emit.AppCallNoArgs(w.BinWriter, stdHash, "serialize", callflag.All) // 39 bytes
	emit.String(w.BinWriter, "lastOracleResponse")
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStoragePut)
	emit.Opcodes(w.BinWriter, opcode.RET)

	m := manifest.NewManifest("TestOracle")
	m.ABI.Methods = []manifest.Method{
		{
			Name:   "requestURL",
			Offset: 0,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("url", smartcontract.StringType),
				manifest.NewParameter("filter", smartcontract.StringType),
				manifest.NewParameter("callback", smartcontract.StringType),
				manifest.NewParameter("userData", smartcontract.AnyType),
				manifest.NewParameter("gasForResponse", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "handle",
			Offset: offset,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("url", smartcontract.StringType),
				manifest.NewParameter("userData", smartcontract.AnyType),
				manifest.NewParameter("code", smartcontract.IntegerType),
				manifest.NewParameter("result", smartcontract.ByteArrayType),
			},
			ReturnType: smartcontract.VoidType,
		},
	}

	perm := manifest.NewPermission(manifest.PermissionHash, oracleHash)
	perm.Methods.Add("request")
	m.Permissions = append(m.Permissions, *perm)

	// Generate NEF file.
	script := w.Bytes()
	ne, err := nef.NewFile(script)
	require.NoError(t, err)

	// Write NEF file.
	bytes, err := ne.Bytes()
	require.NoError(t, err)
	if saveState {
		err = ioutil.WriteFile(oracleContractNEFPath, bytes, os.ModePerm)
		require.NoError(t, err)
	}

	// Write manifest file.
	mData, err := json.Marshal(m)
	require.NoError(t, err)
	if saveState {
		err = ioutil.WriteFile(oracleContractManifestPath, mData, os.ModePerm)
		require.NoError(t, err)
	}

	require.False(t, saveState)
}

// getOracleContractState reads pre-compiled oracle contract generated by
// TestGenerateOracleContract and returns its state.
func getOracleContractState(t *testing.T, sender util.Uint160, id int32) *state.Contract {
	errNotFound := errors.New("auto-generated oracle contract is not found, use TestGenerateOracleContract to regenerate")

	neBytes, err := ioutil.ReadFile(oracleContractNEFPath)
	require.NoError(t, err, fmt.Errorf("nef: %w", errNotFound))
	ne, err := nef.FileFromBytes(neBytes)
	require.NoError(t, err)

	mBytes, err := ioutil.ReadFile(oracleContractManifestPath)
	require.NoError(t, err, fmt.Errorf("manifest: %w", errNotFound))
	m := &manifest.Manifest{}
	err = json.Unmarshal(mBytes, m)
	require.NoError(t, err)

	return &state.Contract{
		ContractBase: state.ContractBase{
			NEF:      ne,
			Hash:     state.CreateContractHash(sender, ne.Checksum, m.Name),
			Manifest: *m,
			ID:       id,
		},
	}
}

func putOracleRequest(t *testing.T, h util.Uint160, bc *Blockchain,
	url string, filter *string, cb string, userData []byte, gas int64) util.Uint256 {
	var filtItem interface{}
	if filter != nil {
		filtItem = *filter
	}
	res, err := invokeContractMethod(bc, gas+50_000_000+5_000_000, h, "requestURL",
		url, filtItem, cb, userData, gas)
	require.NoError(t, err)
	return res.Container
}

func getOracleConfig(t *testing.T, bc *Blockchain, w, pass string) oracle.Config {
	return oracle.Config{
		Log:     zaptest.NewLogger(t),
		Network: netmode.UnitTestNet,
		MainCfg: config.OracleConfiguration{
			RefreshInterval:     time.Second,
			AllowedContentTypes: []string{"application/json"},
			UnlockWallet: config.Wallet{
				Path:     filepath.Join(oracleModulePath, w),
				Password: pass,
			},
		},
		Chain:  bc,
		Client: newDefaultHTTPClient(),
	}
}

func getTestOracle(t *testing.T, bc *Blockchain, walletPath, pass string) (
	*wallet.Account,
	*oracle.Oracle,
	map[uint64]*responseWithSig,
	chan *transaction.Transaction) {
	m := make(map[uint64]*responseWithSig)
	ch := make(chan *transaction.Transaction, 5)
	orcCfg := getOracleConfig(t, bc, walletPath, pass)
	orcCfg.ResponseHandler = &saveToMapBroadcaster{m: m}
	orcCfg.OnTransaction = saveTxToChan(ch)
	orcCfg.URIValidator = func(u *url.URL) error {
		if strings.HasPrefix(u.Host, "private") {
			return errors.New("private network")
		}
		return nil
	}
	orc, err := oracle.NewOracle(orcCfg)
	require.NoError(t, err)

	w, err := wallet.NewWalletFromFile(path.Join(oracleModulePath, walletPath))
	require.NoError(t, err)
	require.NoError(t, w.Accounts[0].Decrypt(pass, w.Scrypt))
	return w.Accounts[0], orc, m, ch
}

// Compatibility test from C# code.
// https://github.com/neo-project/neo-modules/blob/master/tests/Neo.Plugins.OracleService.Tests/UT_OracleService.cs#L61
func TestCreateResponseTx(t *testing.T) {
	bc := newTestChain(t)

	require.Equal(t, int64(30), bc.GetBaseExecFee())
	require.Equal(t, int64(1000), bc.FeePerByte())
	acc, orc, _, _ := getTestOracle(t, bc, "./testdata/oracle1.json", "one")
	req := &state.OracleRequest{
		OriginalTxID:     util.Uint256{},
		GasForResponse:   100000000,
		URL:              "https://127.0.0.1/test",
		Filter:           new(string),
		CallbackContract: util.Uint160{},
		CallbackMethod:   "callback",
		UserData:         []byte{},
	}
	resp := &transaction.OracleResponse{
		ID:     1,
		Code:   transaction.Success,
		Result: []byte{0},
	}
	require.NoError(t, bc.contracts.Oracle.PutRequestInternal(1, req, bc.dao))
	orc.UpdateOracleNodes(keys.PublicKeys{acc.PrivateKey().PublicKey()})
	bc.SetOracle(orc)
	tx, err := orc.CreateResponseTx(int64(req.GasForResponse), 1, resp)
	require.NoError(t, err)
	assert.Equal(t, 166, tx.Size())
	assert.Equal(t, int64(2198650), tx.NetworkFee)
	assert.Equal(t, int64(97801350), tx.SystemFee)
}

func TestOracle_InvalidWallet(t *testing.T) {
	bc := newTestChain(t)

	_, err := oracle.NewOracle(getOracleConfig(t, bc, "./testdata/oracle1.json", "invalid"))
	require.Error(t, err)

	_, err = oracle.NewOracle(getOracleConfig(t, bc, "./testdata/oracle1.json", "one"))
	require.NoError(t, err)
}

func TestOracle(t *testing.T) {
	bc := newTestChain(t)

	oracleCtr := bc.contracts.Oracle
	acc1, orc1, m1, ch1 := getTestOracle(t, bc, "./testdata/oracle1.json", "one")
	acc2, orc2, m2, ch2 := getTestOracle(t, bc, "./testdata/oracle2.json", "two")
	oracleNodes := keys.PublicKeys{acc1.PrivateKey().PublicKey(), acc2.PrivateKey().PublicKey()}
	// Must be set in native contract for tx verification.
	bc.setNodesByRole(t, true, noderoles.Oracle, oracleNodes)
	orc1.UpdateOracleNodes(oracleNodes.Copy())
	orc2.UpdateOracleNodes(oracleNodes.Copy())

	orcNative := bc.contracts.Oracle
	md, ok := orcNative.GetMethod(manifest.MethodVerify, -1)
	require.True(t, ok)
	orc1.UpdateNativeContract(orcNative.NEF.Script, orcNative.GetOracleResponseScript(), orcNative.Hash, md.MD.Offset)
	orc2.UpdateNativeContract(orcNative.NEF.Script, orcNative.GetOracleResponseScript(), orcNative.Hash, md.MD.Offset)

	cs := getOracleContractState(t, util.Uint160{}, 42)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))

	putOracleRequest(t, cs.Hash, bc, "https://get.1234", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://get.1234", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://get.timeout", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://get.notfound", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://get.forbidden", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://private.url", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://get.big", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://get.maxallowed", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://get.maxallowed", nil, "handle", []byte{}, 100_000_000)

	flt := "$.Values[1]"
	putOracleRequest(t, cs.Hash, bc, "https://get.filter", &flt, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "https://get.filterinv", &flt, "handle", []byte{}, 10_000_000)

	putOracleRequest(t, cs.Hash, bc, "https://get.invalidcontent", nil, "handle", []byte{}, 10_000_000)

	checkResp := func(t *testing.T, id uint64, resp *transaction.OracleResponse) *state.OracleRequest {
		req, err := oracleCtr.GetRequestInternal(bc.dao, id)
		require.NoError(t, err)

		reqs := map[uint64]*state.OracleRequest{id: req}
		orc1.ProcessRequestsInternal(reqs)
		require.NotNil(t, m1[id])
		require.Equal(t, resp, m1[id].resp)
		require.Empty(t, ch1)
		return req
	}

	// Checks if tx is ready and valid.
	checkEmitTx := func(t *testing.T, ch chan *transaction.Transaction) {
		require.Len(t, ch, 1)
		tx := <-ch

		// Response transaction has its hash being precalculated. Check that this hash
		// matches the actual one.
		cachedHash := tx.Hash()
		cp := transaction.Transaction{
			Version:         tx.Version,
			Nonce:           tx.Nonce,
			SystemFee:       tx.SystemFee,
			NetworkFee:      tx.NetworkFee,
			ValidUntilBlock: tx.ValidUntilBlock,
			Script:          tx.Script,
			Attributes:      tx.Attributes,
			Signers:         tx.Signers,
			Scripts:         tx.Scripts,
			Trimmed:         tx.Trimmed,
		}
		actualHash := cp.Hash()
		require.Equal(t, actualHash, cachedHash, "transaction hash was changed during ")

		require.NoError(t, bc.verifyAndPoolTx(tx, bc.GetMemPool(), bc))
	}

	t.Run("NormalRequest", func(t *testing.T) {
		resp := &transaction.OracleResponse{
			ID:     0,
			Code:   transaction.Success,
			Result: []byte{1, 2, 3, 4},
		}
		req := checkResp(t, 0, resp)

		reqs := map[uint64]*state.OracleRequest{0: req}
		orc2.ProcessRequestsInternal(reqs)
		require.Equal(t, resp, m2[0].resp)
		require.Empty(t, ch2)

		t.Run("InvalidSignature", func(t *testing.T) {
			orc1.AddResponse(acc2.PrivateKey().PublicKey(), m2[0].resp.ID, []byte{1, 2, 3})
			require.Empty(t, ch1)
		})
		orc1.AddResponse(acc2.PrivateKey().PublicKey(), m2[0].resp.ID, m2[0].txSig)
		checkEmitTx(t, ch1)

		t.Run("FirstOtherThenMe", func(t *testing.T) {
			const reqID = 1

			resp := &transaction.OracleResponse{
				ID:     reqID,
				Code:   transaction.Success,
				Result: []byte{1, 2, 3, 4},
			}
			req := checkResp(t, reqID, resp)
			orc2.AddResponse(acc1.PrivateKey().PublicKey(), reqID, m1[reqID].txSig)
			require.Empty(t, ch2)

			reqs := map[uint64]*state.OracleRequest{reqID: req}
			orc2.ProcessRequestsInternal(reqs)
			require.Equal(t, resp, m2[reqID].resp)
			checkEmitTx(t, ch2)
		})
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("Timeout", func(t *testing.T) {
			checkResp(t, 2, &transaction.OracleResponse{
				ID:   2,
				Code: transaction.Timeout,
			})
		})
		t.Run("NotFound", func(t *testing.T) {
			checkResp(t, 3, &transaction.OracleResponse{
				ID:   3,
				Code: transaction.NotFound,
			})
		})
		t.Run("Forbidden", func(t *testing.T) {
			checkResp(t, 4, &transaction.OracleResponse{
				ID:   4,
				Code: transaction.Forbidden,
			})
		})
		t.Run("PrivateNetwork", func(t *testing.T) {
			checkResp(t, 5, &transaction.OracleResponse{
				ID:   5,
				Code: transaction.Forbidden,
			})
		})
		t.Run("Big", func(t *testing.T) {
			checkResp(t, 6, &transaction.OracleResponse{
				ID:   6,
				Code: transaction.ResponseTooLarge,
			})
		})
		t.Run("MaxAllowedSmallGAS", func(t *testing.T) {
			checkResp(t, 7, &transaction.OracleResponse{
				ID:   7,
				Code: transaction.InsufficientFunds,
			})
		})
	})
	t.Run("MaxAllowedEnoughGAS", func(t *testing.T) {
		checkResp(t, 8, &transaction.OracleResponse{
			ID:     8,
			Code:   transaction.Success,
			Result: make([]byte, transaction.MaxOracleResultSize),
		})
	})
	t.Run("WithFilter", func(t *testing.T) {
		checkResp(t, 9, &transaction.OracleResponse{
			ID:     9,
			Code:   transaction.Success,
			Result: []byte(`[2]`),
		})
		t.Run("invalid response", func(t *testing.T) {
			checkResp(t, 10, &transaction.OracleResponse{
				ID:   10,
				Code: transaction.Error,
			})
		})
	})
	t.Run("InvalidContentType", func(t *testing.T) {
		checkResp(t, 11, &transaction.OracleResponse{
			ID:   11,
			Code: transaction.ContentTypeNotSupported,
		})
	})
}

func TestOracleFull(t *testing.T) {
	bc := initTestChain(t, nil, nil)
	acc, orc, _, _ := getTestOracle(t, bc, "./testdata/oracle2.json", "two")
	mp := bc.GetMemPool()
	orc.OnTransaction = func(tx *transaction.Transaction) { _ = mp.Add(tx, bc) }
	bc.SetOracle(orc)

	cs := getOracleContractState(t, util.Uint160{}, 42)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))

	go bc.Run()
	go orc.Run()
	t.Cleanup(orc.Shutdown)

	bc.setNodesByRole(t, true, noderoles.Oracle, keys.PublicKeys{acc.PrivateKey().PublicKey()})
	putOracleRequest(t, cs.Hash, bc, "https://get.1234", new(string), "handle", []byte{}, 10_000_000)

	require.Eventually(t, func() bool { return mp.Count() == 1 },
		time.Second*3, time.Millisecond*200)

	txes := mp.GetVerifiedTransactions()
	require.Len(t, txes, 1)
	require.True(t, txes[0].HasAttribute(transaction.OracleResponseT))
}

func TestNotYetRunningOracle(t *testing.T) {
	bc := initTestChain(t, nil, nil)
	acc, orc, _, _ := getTestOracle(t, bc, "./testdata/oracle2.json", "two")
	mp := bc.GetMemPool()
	orc.OnTransaction = func(tx *transaction.Transaction) { _ = mp.Add(tx, bc) }
	bc.SetOracle(orc)

	cs := getOracleContractState(t, util.Uint160{}, 42)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))

	go bc.Run()
	bc.setNodesByRole(t, true, noderoles.Oracle, keys.PublicKeys{acc.PrivateKey().PublicKey()})

	var req state.OracleRequest
	var reqs = make(map[uint64]*state.OracleRequest)
	for i := uint64(0); i < 3; i++ {
		reqs[i] = &req
	}
	orc.AddRequests(reqs) // 0, 1, 2 added to pending.

	var ids = []uint64{0, 1}
	orc.RemoveRequests(ids) // 0, 1 removed from pending, 2 left.

	reqs = make(map[uint64]*state.OracleRequest)
	for i := uint64(3); i < 5; i++ {
		reqs[i] = &req
	}
	orc.AddRequests(reqs) // 3, 4 added to pending -> 2, 3, 4 in pending.

	ids = []uint64{3}
	orc.RemoveRequests(ids) // 3 removed from pending -> 2, 4 in pending.

	go orc.Run()
	t.Cleanup(orc.Shutdown)

	require.Eventually(t, func() bool { return mp.Count() == 2 },
		time.Second*3, time.Millisecond*200)
	txes := mp.GetVerifiedTransactions()
	require.Len(t, txes, 2)
	var txids []uint64
	for _, tx := range txes {
		for _, attr := range tx.Attributes {
			if attr.Type == transaction.OracleResponseT {
				resp := attr.Value.(*transaction.OracleResponse)
				txids = append(txids, resp.ID)
			}
		}
	}
	require.Len(t, txids, 2)
	require.Contains(t, txids, uint64(2))
	require.Contains(t, txids, uint64(4))
}

type saveToMapBroadcaster struct {
	mtx sync.RWMutex
	m   map[uint64]*responseWithSig
}

func (b *saveToMapBroadcaster) SendResponse(_ *keys.PrivateKey, resp *transaction.OracleResponse, txSig []byte) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.m[resp.ID] = &responseWithSig{
		resp:  resp,
		txSig: txSig,
	}
}
func (*saveToMapBroadcaster) Run()      {}
func (*saveToMapBroadcaster) Shutdown() {}

type responseWithSig struct {
	resp  *transaction.OracleResponse
	txSig []byte
}

func saveTxToChan(ch chan *transaction.Transaction) oracle.TxCallback {
	return func(tx *transaction.Transaction) {
		ch <- tx
	}
}

type (
	// httpClient implements oracle.HTTPClient with
	// mocked URL or responses.
	httpClient struct {
		responses map[string]testResponse
	}

	testResponse struct {
		code int
		ct   string
		body []byte
	}
)

// Get implements oracle.HTTPClient interface.
func (c *httpClient) Do(req *http.Request) (*http.Response, error) {
	resp, ok := c.responses[req.URL.String()]
	if ok {
		return &http.Response{
			StatusCode: resp.code,
			Header: http.Header{
				"Content-Type": {resp.ct},
			},
			Body: newResponseBody(resp.body),
		}, nil
	}
	return nil, errors.New("request failed")
}

func newDefaultHTTPClient() oracle.HTTPClient {
	return &httpClient{
		responses: map[string]testResponse{
			"https://get.1234": {
				code: http.StatusOK,
				ct:   "application/json",
				body: []byte{1, 2, 3, 4},
			},
			"https://get.4321": {
				code: http.StatusOK,
				ct:   "application/json",
				body: []byte{4, 3, 2, 1},
			},
			"https://get.timeout": {
				code: http.StatusRequestTimeout,
				ct:   "application/json",
				body: []byte{},
			},
			"https://get.notfound": {
				code: http.StatusNotFound,
				ct:   "application/json",
				body: []byte{},
			},
			"https://get.forbidden": {
				code: http.StatusForbidden,
				ct:   "application/json",
				body: []byte{},
			},
			"https://private.url": {
				code: http.StatusOK,
				ct:   "application/json",
				body: []byte("passwords"),
			},
			"https://get.big": {
				code: http.StatusOK,
				ct:   "application/json",
				body: make([]byte, transaction.MaxOracleResultSize+1),
			},
			"https://get.maxallowed": {
				code: http.StatusOK,
				ct:   "application/json",
				body: make([]byte, transaction.MaxOracleResultSize),
			},
			"https://get.filter": {
				code: http.StatusOK,
				ct:   "application/json",
				body: []byte(`{"Values":["one", 2, 3],"Another":null}`),
			},
			"https://get.filterinv": {
				code: http.StatusOK,
				ct:   "application/json",
				body: []byte{0xFF},
			},
			"https://get.invalidcontent": {
				code: http.StatusOK,
				ct:   "image/gif",
				body: []byte{1, 2, 3},
			},
		},
	}
}

func newResponseBody(resp []byte) gio.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(resp))
}
