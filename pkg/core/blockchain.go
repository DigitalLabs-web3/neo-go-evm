package core

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/consensus"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/blockchainer"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/filters"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/interop"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/mempool"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/mpt"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/noderoles"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/statedb"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/stateroot"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/vm"
	evm "github.com/DigitalLabs-web3/neo-go-evm/pkg/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"go.uber.org/zap"
)

// Tuning parameters.
const (
	headerBatchCount = 2000
	version          = "0.2.5"

	defaultInitialGAS                      = 52000000 //wei
	defaultGCPeriod                        = 10000
	defaultMemPoolSize                     = 50000
	defaultP2PNotaryRequestPayloadPoolSize = 1000
	defaultMaxBlockSize                    = 262144
	defaultMaxBlockGas                     = 90000000000
	defaultMaxTraceableBlocks              = 2102400 // 1 year of 15s blocks
	defaultMaxTransactionsPerBlock         = 512
	// HeaderVerificationGasLimit is the maximum amount of GAS for block header verification.
	HeaderVerificationGasLimit = 3_00000000 // 3 GAS
	defaultStateSyncInterval   = 40000
)

var (
	// ErrAlreadyExists is returned when trying to add some already existing
	// transaction into the pool (not specifying whether it exists in the
	// chain or mempool).
	ErrAlreadyExists = errors.New("already exists")
	// ErrOOM is returned when adding transaction to the memory pool because
	// it reached its full capacity.
	ErrOOM = errors.New("no space left in the memory pool")
	// ErrPolicy is returned on attempt to add transaction that doesn't
	// comply with node's configured policy into the mempool.
	ErrPolicy = errors.New("not allowed by policy")
	// ErrInvalidBlockIndex is returned when trying to add block with index
	// other than expected height of the blockchain.
	ErrInvalidBlockIndex = errors.New("invalid block index")
	// ErrHasConflicts is returned when trying to add some transaction which
	// conflicts with other transaction in the chain or pool according to
	// Conflicts attribute.
	ErrHasConflicts = errors.New("has conflicts")
)
var (
	persistInterval = 1 * time.Second
)

// Blockchain represents the blockchain. It maintans internal state representing
// the state of the ledger that can be accessed in various ways and changed by
// adding new blocks or headers.
type Blockchain struct {
	config config.ProtocolConfiguration

	// The only way chain state changes is by adding blocks, so we can't
	// allow concurrent block additions. It differs from the next lock in
	// that it's only for AddBlock method itself, the chain state is
	// protected by the lock below, but holding it during all of AddBlock
	// is too expensive (because the state only changes when persisting
	// change cache).
	addLock sync.Mutex

	// This lock ensures blockchain immutability for operations that need
	// that while performing their tasks. It's mostly used as a read lock
	// with the only writer being the block addition logic.
	lock sync.RWMutex

	// Data access object for CRUD operations around storage. It's write-cached.
	dao *dao.Simple

	// persistent is the same DB as dao, but we never write to it, so all reads
	// are directly from underlying persistent store.
	persistent *dao.Simple

	// Underlying persistent store.
	store storage.Store

	// Current index/height of the highest block.
	// Read access should always be called by BlockHeight().
	// Write access should only happen in storeBlock().
	blockHeight uint32

	// Current top Block wrapped in an atomic.Value for safe access.
	topBlock    atomic.Value
	topBlockAer atomic.Value

	// Current persisted block count.
	persistedHeight uint32

	// Number of headers stored in the chain file.
	storedHeaderCount uint32

	// Header hashes list with associated lock.
	headerHashesLock sync.RWMutex
	headerHashes     []common.Hash

	// Stop synchronization mechanisms.
	stopCh      chan struct{}
	runToExitCh chan struct{}

	memPool *mempool.Pool

	// postBlock is a set of callback methods which should be run under the Blockchain lock after new block is persisted.
	// Block's transactions are passed via mempool.
	postBlock []func(func(*transaction.Transaction, *mempool.Pool, bool) bool, *mempool.Pool, *block.Block)

	log *zap.Logger

	lastBatch *storage.MemBatch

	contracts native.Contracts

	extensible atomic.Value

	// knownValidatorsCount is the latest known validators count used
	// for defaultBlockWitness.
	knownValidatorsCount atomic.Value
	// defaultBlockWitness stores transaction.Witness with m out of n multisig,
	// where n = knownValidatorsCount.
	defaultBlockWitness atomic.Value

	stateRoot *stateroot.Module

	// Notification subsystem.
	events  chan bcEvent
	subCh   chan interface{}
	unsubCh chan interface{}
}

// bcEvent is an internal event generated by the Blockchain and then
// broadcasted to other parties. It joins the new block and associated
// invocation logs, all the other events visible from outside can be produced
// from this combination.
type bcEvent struct {
	block          *block.Block
	appExecResults []*types.Receipt
}

// NewBlockchain returns a new blockchain object the will use the
// given Store as its underlying storage. For it to work correctly you need
// to spawn a goroutine for its Run method after this initialization.
func NewBlockchain(s storage.Store, cfg config.ProtocolConfiguration, log *zap.Logger) (*Blockchain, error) {
	if log == nil {
		return nil, errors.New("empty logger")
	}

	if cfg.InitialGASSupply <= 0 {
		cfg.InitialGASSupply = defaultInitialGAS
		log.Info("initial gas supply is not set or wrong, setting default value", zap.Uint64("InitialGASSupply", cfg.InitialGASSupply))
	}
	if cfg.MemPoolSize <= 0 {
		cfg.MemPoolSize = defaultMemPoolSize
		log.Info("mempool size is not set or wrong, setting default value", zap.Int("MemPoolSize", cfg.MemPoolSize))
	}
	if cfg.MaxBlockSize == 0 {
		cfg.MaxBlockSize = defaultMaxBlockSize
		log.Info("MaxBlockSize is not set or wrong, setting default value", zap.Uint32("MaxBlockSize", cfg.MaxBlockSize))
	}
	if cfg.MaxBlockGas <= 0 {
		cfg.MaxBlockGas = defaultMaxBlockGas
		log.Info("MaxBlockSystemFee is not set or wrong, setting default value", zap.Uint64("MaxBlockSystemFee", cfg.MaxBlockGas))
	}
	if cfg.MaxTraceableBlocks == 0 {
		cfg.MaxTraceableBlocks = defaultMaxTraceableBlocks
		log.Info("MaxTraceableBlocks is not set or wrong, using default value", zap.Uint32("MaxTraceableBlocks", cfg.MaxTraceableBlocks))
	}
	if cfg.MaxTransactionsPerBlock == 0 {
		cfg.MaxTransactionsPerBlock = defaultMaxTransactionsPerBlock
		log.Info("MaxTransactionsPerBlock is not set or wrong, using default value",
			zap.Uint16("MaxTransactionsPerBlock", cfg.MaxTransactionsPerBlock))
	}

	if cfg.RemoveUntraceableBlocks && cfg.GarbageCollectionPeriod == 0 {
		cfg.GarbageCollectionPeriod = defaultGCPeriod
		log.Info("GarbageCollectionPeriod is not set or wrong, using default value", zap.Uint32("GarbageCollectionPeriod", cfg.GarbageCollectionPeriod))
	}
	bc := &Blockchain{
		config:      cfg,
		dao:         dao.NewSimple(s),
		persistent:  dao.NewSimple(s),
		store:       s,
		stopCh:      make(chan struct{}),
		runToExitCh: make(chan struct{}),
		memPool:     mempool.New(cfg.MemPoolSize, 0, false),
		log:         log,
		events:      make(chan bcEvent),
		subCh:       make(chan interface{}),
		unsubCh:     make(chan interface{}),
		contracts:   *native.NewContracts(cfg),
	}

	bc.stateRoot = stateroot.NewModule(bc.GetConfig(), bc.VerifyWitness, bc.log, bc.dao.Store)

	if err := bc.init(); err != nil {
		return nil, err
	}

	return bc, nil
}

func (bc *Blockchain) init() error {
	// If we could not find the version in the Store, we know that there is nothing stored.
	ver, err := bc.dao.GetVersion()
	if err != nil {
		bc.log.Info("no storage version found! creating genesis block")
		ver = dao.Version{
			StoragePrefix:       storage.STStorage,
			KeepOnlyLatestState: bc.config.KeepOnlyLatestState,
			Value:               version,
		}
		bc.dao.PutVersion(ver)
		bc.dao.Version = ver
		bc.persistent.Version = ver
		genesisBlock, err := createGenesisBlock(&bc.config)
		if err != nil {
			return err
		}
		bc.headerHashes = []common.Hash{genesisBlock.Hash()}
		bc.dao.PutCurrentHeader(genesisBlock.Hash(), genesisBlock.Index)
		if err := bc.stateRoot.Init(0); err != nil {
			return fmt.Errorf("can't init MPT: %w", err)
		}
		return bc.storeBlock(genesisBlock, nil)
	}
	if ver.Value != version {
		return fmt.Errorf("storage version mismatch (expected=%s, actual=%s)", version, ver.Value)
	}
	if ver.KeepOnlyLatestState != bc.config.KeepOnlyLatestState {
		return fmt.Errorf("KeepOnlyLatestState setting mismatch (old=%v, new=%v)",
			ver.KeepOnlyLatestState, bc.config.KeepOnlyLatestState)
	}
	bc.dao.Version = ver
	bc.persistent.Version = ver

	// At this point there was no version found in the storage which
	// implies a creating fresh storage with the version specified
	// and the genesis block as first block.
	bc.log.Info("restoring blockchain", zap.String("version", version))

	bc.headerHashes, err = bc.dao.GetHeaderHashes()
	if err != nil {
		return err
	}

	bc.storedHeaderCount = uint32(len(bc.headerHashes))

	currHeaderHeight, currHeaderHash, err := bc.dao.GetCurrentHeaderHeight()
	if err != nil {
		return fmt.Errorf("failed to retrieve current header info: %w", err)
	}
	if bc.storedHeaderCount == 0 && currHeaderHeight == 0 {
		bc.headerHashes = append(bc.headerHashes, currHeaderHash)
	}
	// There is a high chance that the Node is stopped before the next
	// batch of 2000 headers was stored. Via the currentHeaders stored we can sync
	// that with stored blocks.
	if currHeaderHeight >= bc.storedHeaderCount {
		hash := currHeaderHash
		var targetHash common.Hash
		if len(bc.headerHashes) > 0 {
			targetHash = bc.headerHashes[len(bc.headerHashes)-1]
		} else {
			genesisBlock, err := createGenesisBlock(&bc.config)
			if err != nil {
				return err
			}
			targetHash = genesisBlock.Hash()
			bc.headerHashes = append(bc.headerHashes, targetHash)
		}
		headers := make([]*block.Header, 0)

		for hash != targetHash {
			header, err := bc.GetHeader(hash)
			if err != nil {
				return fmt.Errorf("could not get header %s: %w", hash, err)
			}
			headers = append(headers, header)
			hash = header.PrevHash
		}
		headerSliceReverse(headers)
		for _, h := range headers {
			bc.headerHashes = append(bc.headerHashes, h.Hash())
		}
	}

	bHeight, err := bc.dao.GetCurrentBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to retrieve current block height: %w", err)
	}
	bc.blockHeight = bHeight
	bc.persistedHeight = bHeight
	if err = bc.stateRoot.Init(bHeight); err != nil {
		return fmt.Errorf("can't init MPT at height %d: %w", bHeight, err)
	}

	err = bc.initializeNativeCache(bc.blockHeight, bc.dao)
	if err != nil {
		return fmt.Errorf("can't init natives cache: %w", err)
	}

	return bc.updateExtensibleWhitelist(bHeight)
}

func (bc *Blockchain) initializeNativeCache(blockHeight uint32, d *dao.Simple) error {
	err := bc.contracts.Designate.UpdateCache(d)
	if err != nil {
		return fmt.Errorf("can't init cache for Designation native contract: %w", err)
	}
	err = bc.contracts.Policy.UpdateCache(d)
	if err != nil {
		return fmt.Errorf("can't init cache for Policy native contract: %w", err)
	}
	return nil
}

// Run runs chain loop, it needs to be run as goroutine and executing it is
// critical for correct Blockchain operation.
func (bc *Blockchain) Run() {
	persistTimer := time.NewTimer(persistInterval)
	defer func() {
		persistTimer.Stop()
		if _, err := bc.persist(true); err != nil {
			bc.log.Warn("failed to persist", zap.Error(err))
		}
		if err := bc.dao.Store.Close(); err != nil {
			bc.log.Warn("failed to close db", zap.Error(err))
		}
		close(bc.runToExitCh)
	}()
	go bc.notificationDispatcher()
	var nextSync bool
	for {
		select {
		case <-bc.stopCh:
			return
		case <-persistTimer.C:
			var oldPersisted uint32
			var gcDur time.Duration

			if bc.config.RemoveUntraceableBlocks {
				oldPersisted = atomic.LoadUint32(&bc.persistedHeight)
			}
			dur, err := bc.persist(nextSync)
			if err != nil {
				bc.log.Warn("failed to persist blockchain", zap.Error(err))
			}
			if bc.config.RemoveUntraceableBlocks {
				gcDur = bc.tryRunGC(oldPersisted)
			}
			nextSync = dur > persistInterval*2
			interval := persistInterval - dur - gcDur
			if interval <= 0 {
				interval = time.Microsecond // Reset doesn't work with zero value
			}
			persistTimer.Reset(interval)
		}
	}
}

func (bc *Blockchain) tryRunGC(old uint32) time.Duration {
	var dur time.Duration

	new := atomic.LoadUint32(&bc.persistedHeight)
	var tgtBlock = int64(new)

	tgtBlock -= int64(bc.config.MaxTraceableBlocks)
	// Always round to the GCP.
	tgtBlock /= int64(bc.config.GarbageCollectionPeriod)
	tgtBlock *= int64(bc.config.GarbageCollectionPeriod)
	// Count periods.
	old /= bc.config.GarbageCollectionPeriod
	new /= bc.config.GarbageCollectionPeriod
	if tgtBlock > int64(bc.config.GarbageCollectionPeriod) && new != old {
		tgtBlock /= int64(bc.config.GarbageCollectionPeriod)
		tgtBlock *= int64(bc.config.GarbageCollectionPeriod)
		dur = bc.stateRoot.GC(uint32(tgtBlock), bc.store)
	}
	return dur
}

// notificationDispatcher manages subscription to events and broadcasts new events.
func (bc *Blockchain) notificationDispatcher() {
	var (
		// These are just sets of subscribers, though modelled as maps
		// for ease of management (not a lot of subscriptions is really
		// expected, but maps are convenient for adding/deleting elements).
		blockFeed        = make(map[chan<- *block.Block]bool)
		txFeed           = make(map[chan<- *transaction.Transaction]bool)
		notificationFeed = make(map[chan<- *types.Log]bool)
		executionFeed    = make(map[chan<- *types.Receipt]bool)
	)
	for {
		select {
		case <-bc.stopCh:
			return
		case sub := <-bc.subCh:
			switch ch := sub.(type) {
			case chan<- *block.Block:
				blockFeed[ch] = true
			case chan<- *transaction.Transaction:
				txFeed[ch] = true
			case chan<- *types.Log:
				notificationFeed[ch] = true
			case chan<- *types.Receipt:
				executionFeed[ch] = true
			default:
				panic(fmt.Sprintf("bad subscription: %T", sub))
			}
		case unsub := <-bc.unsubCh:
			switch ch := unsub.(type) {
			case chan<- *block.Block:
				delete(blockFeed, ch)
			case chan<- *transaction.Transaction:
				delete(txFeed, ch)
			case chan<- *types.Log:
				delete(notificationFeed, ch)
			case chan<- *types.Receipt:
				delete(executionFeed, ch)
			default:
				panic(fmt.Sprintf("bad unsubscription: %T", unsub))
			}
		case event := <-bc.events:
			// We don't want to waste time looping through transactions when there are no
			// subscribers.
			if len(txFeed) != 0 || len(notificationFeed) != 0 || len(executionFeed) != 0 {
				for i, tx := range event.block.Transactions {
					aer := event.appExecResults[i]
					if aer.TxHash != (tx.Hash()) {
						panic("inconsistent application execution results")
					}
					for ch := range executionFeed {
						ch <- aer
					}
					if aer.Status == 1 {
						for _, log := range aer.Logs {
							for ch := range notificationFeed {
								ch <- log
							}
						}
					}
					for ch := range txFeed {
						ch <- tx
					}
				}
			}
			for ch := range blockFeed {
				ch <- event.block
			}
		}
	}
}

// Close stops Blockchain's internal loop, syncs changes to persistent storage
// and closes it. The Blockchain is no longer functional after the call to Close.
func (bc *Blockchain) Close() {
	// If there is a block addition in progress, wait for it to finish and
	// don't allow new ones.
	bc.addLock.Lock()
	close(bc.stopCh)
	<-bc.runToExitCh
	bc.addLock.Unlock()
}

// AddBlock accepts successive block for the Blockchain, verifies it and
// stores internally. Eventually it will be persisted to the backing storage.
func (bc *Blockchain) AddBlock(block *block.Block) error {
	bc.addLock.Lock()
	defer bc.addLock.Unlock()

	var mp *mempool.Pool
	expectedHeight := bc.BlockHeight() + 1
	if expectedHeight != block.Index {
		return fmt.Errorf("expected %d, got %d: %w", expectedHeight, block.Index, ErrInvalidBlockIndex)
	}

	if block.Index == bc.HeaderHeight()+1 {
		err := bc.addHeaders(bc.config.VerifyBlocks, &block.Header)
		if err != nil {
			return err
		}
	}
	if bc.config.VerifyBlocks {
		merkle := block.ComputeMerkleRoot()
		if block.MerkleRoot != merkle {
			return errors.New("invalid block: MerkleRoot mismatch")
		}
		mp = mempool.New(len(block.Transactions), 0, false)
		for _, tx := range block.Transactions {
			var err error
			// Transactions are verified before adding them
			// into the pool, so there is no point in doing
			// it again even if we're verifying in-block transactions.
			if bc.memPool.ContainsKey(tx.Hash()) {
				err = mp.Add(tx, bc)
				if err == nil {
					continue
				}
			} else {
				err = bc.verifyAndPoolTx(tx, mp, bc)
			}
			if err != nil && bc.config.VerifyTransactions {
				return fmt.Errorf("transaction %s failed to verify: %w", tx.Hash().String(), err)
			}
		}
	}
	return bc.storeBlock(block, mp)
}

// AddHeaders processes the given headers and add them to the
// HeaderHashList. It expects headers to be sorted by index.
func (bc *Blockchain) AddHeaders(headers ...*block.Header) error {
	return bc.addHeaders(bc.config.VerifyBlocks, headers...)
}

// addHeaders is an internal implementation of AddHeaders (`verify` parameter
// tells it to verify or not verify given headers).
func (bc *Blockchain) addHeaders(verify bool, headers ...*block.Header) error {
	var (
		start = time.Now()
		batch = bc.dao.GetPrivate()
		err   error
	)

	if len(headers) > 0 {
		var i int
		curHeight := bc.HeaderHeight()
		for i = range headers {
			if headers[i].Index > curHeight {
				break
			}
		}
		headers = headers[i:]
	}

	if len(headers) == 0 {
		return nil
	} else if verify {
		// Verify that the chain of the headers is consistent.
		var lastHeader *block.Header
		if lastHeader, err = bc.GetHeader(headers[0].PrevHash); err != nil {
			return fmt.Errorf("previous header was not found: %w", err)
		}
		for _, h := range headers {
			if err = bc.verifyHeader(h, lastHeader); err != nil {
				return err
			}
			lastHeader = h
		}
	}

	bc.headerHashesLock.Lock()
	defer bc.headerHashesLock.Unlock()
	oldlen := len(bc.headerHashes)
	var lastHeader *block.Header
	for _, h := range headers {
		if int(h.Index) != len(bc.headerHashes) {
			continue
		}
		bc.headerHashes = append(bc.headerHashes, h.Hash())
		lastHeader = h
	}

	if oldlen != len(bc.headerHashes) {
		for int(lastHeader.Index)-headerBatchCount >= int(bc.storedHeaderCount) {
			err = batch.StoreHeaderHashes(bc.headerHashes[bc.storedHeaderCount:bc.storedHeaderCount+headerBatchCount],
				bc.storedHeaderCount)
			if err != nil {
				return err
			}
			bc.storedHeaderCount += headerBatchCount
		}

		batch.PutCurrentHeader(lastHeader.Hash(), lastHeader.Index)
		updateHeaderHeightMetric(len(bc.headerHashes) - 1)
		if _, err = batch.Persist(); err != nil {
			return err
		}
		bc.log.Debug("done processing headers",
			zap.Int("headerIndex", len(bc.headerHashes)-1),
			zap.Uint32("blockHeight", bc.BlockHeight()),
			zap.Duration("took", time.Since(start)))
	}
	return nil
}

// GetStateModule returns state root service instance.
func (bc *Blockchain) GetStateModule() blockchainer.StateRoot {
	return bc.stateRoot
}

// storeBlock performs chain update using the block given, it executes all
// transactions with all appropriate side-effects and updates Blockchain state.
// This is the only way to change Blockchain state.
func (bc *Blockchain) storeBlock(block *block.Block, txpool *mempool.Pool) error {
	var (
		cache          = bc.dao.GetPrivate()
		aerCache       = bc.dao.GetPrivate()
		appExecResults = make([]*types.Receipt, 0, 2+len(block.Transactions))
		aerchan        = make(chan *types.Receipt, len(block.Transactions)/8) // Tested 8 and 4 with no practical difference, but feel free to test more and tune.
		aerdone        = make(chan error)
	)
	go func() {
		var (
			kvcache  = aerCache
			err      error
			txCnt    int
			blockaer *types.Receipt
		)
		kvcache.StoreAsCurrentBlock(block)
		if bc.config.RemoveUntraceableBlocks {
			var start, stop uint32
			if block.Index > bc.config.MaxTraceableBlocks {
				start = block.Index - bc.config.MaxTraceableBlocks // is at least 1
				stop = start + 1
			}
			for index := start; index < stop; index++ {
				err := kvcache.DeleteBlock(bc.headerHashes[index])
				if err != nil {
					bc.log.Warn("error while removing old block",
						zap.Uint32("index", index),
						zap.Error(err))
				}
			}
		}
		for aer := range aerchan {
			if txCnt == len(block.Transactions) {
				blockaer = aer
				break
			}
			err = kvcache.StoreAsTransaction(block.Transactions[txCnt], aer)
			txCnt++
			if err != nil {
				err = fmt.Errorf("failed to store exec result: %w", err)
				break
			}
		}
		if err != nil {
			aerdone <- err
			return
		}
		if err := kvcache.StoreAsBlock(block, blockaer); err != nil {
			aerdone <- err
			return
		}
		close(aerdone)
	}()

	var (
		err           error
		execErr       error
		logIndex      uint
		cumulativeGas uint64
		blockGasUsed  uint64
	)
	err = bc.onPersist(cache, block)
	if err != nil {
		return fmt.Errorf("onPersist failed: %w", err)
	}
	sdb := statedb.NewStateDB(cache, bc)
	for i, tx := range block.Transactions {
		bc.log.Debug("executing tx", zap.String("hash", tx.Hash().String()))
		gasPrice := tx.GasPrice()
		blockGasUsed += tx.Gas()
		netFee := transaction.CalculateNetworkFee(tx, bc.FeePerByte())
		gas := tx.Gas() - netFee
		ic, err := interop.NewContext(block, tx, sdb, bc, nil)
		if err != nil {
			panic(err)
		}
		vm := ic.VM
		var (
			left    uint64
			address common.Address
		)
		sdb.PrepareAccessList(tx.From(), tx.To(), evm.PrecompiledAddressesBerlin, tx.AccessList())
		if tx.To() == nil {
			_, address, left, execErr = vm.Create(ic, tx.Data(), gas, tx.Value())
		} else {
			_, left, execErr = vm.Call(ic, *tx.To(), tx.Data(), gas, tx.Value())
		}
		if execErr != nil {
			bc.log.Debug("error when executing tx", zap.Uint32("block_index", block.Index),
				zap.String("tx_hash", tx.Hash().String()),
				zap.String("error", execErr.Error()))
		}
		gasUsed := tx.Gas() - left
		logs := sdb.GetLogs()
		sdb.SetNonce(tx.From(), sdb.GetNonce(tx.From())+1)
		if block.Index > 0 {
			sdb.AddBalance(ic.Coinbase(), big.NewInt(0).Mul(big.NewInt(int64(netFee)), gasPrice))
		}
		if gas > left {
			commitAddress, err := bc.GetConsensusAddress()
			if err != nil {
				panic(err)
			}
			sdb.AddBalance(commitAddress, big.NewInt(0).Mul(big.NewInt(int64(gas-left)), gasPrice))
		}
		refund := sdb.GetRefund()
		maxRefund := gasUsed / params.RefundQuotientEIP3529
		if refund > maxRefund {
			refund = maxRefund
		}
		sdb.SubBalance(tx.From(), big.NewInt(0).Mul(big.NewInt(int64(gasUsed-refund)), gasPrice))
		sdb.Commit()
		cumulativeGas += gasUsed
		for _, log := range logs {
			log.BlockHash = block.Hash()
			log.TxHash = tx.Hash()
			log.TxIndex = uint(i)
			log.Index = logIndex
			logIndex++
		}
		aer := &types.Receipt{
			BlockHash:         block.Hash(),
			BlockNumber:       big.NewInt(int64(block.Index)),
			TxHash:            tx.Hash(),
			TransactionIndex:  uint(i),
			GasUsed:           gasUsed,
			ContractAddress:   address,
			CumulativeGasUsed: cumulativeGas,
			Logs:              logs,
		}
		aer.Bloom = types.BytesToBloom(types.LogsBloom(aer.Logs))
		if execErr == nil {
			aer.Status = 1
		}
		//aer.PostState = []byte{byte(aer.Status)}
		aer.PostState = block.MerkleRoot.Bytes()
		appExecResults = append(appExecResults, aer)
		aerchan <- aer
	}
	err = bc.postPersist(cache, block)
	if err != nil {
		return fmt.Errorf("postPersist failed: %w", err)
	}
	aer := &types.Receipt{
		BlockHash:         block.Hash(),
		BlockNumber:       big.NewInt(int64(block.Index)),
		TxHash:            block.Hash(),
		TransactionIndex:  0,
		GasUsed:           blockGasUsed,
		ContractAddress:   common.Address{},
		CumulativeGasUsed: cumulativeGas,
		Logs:              []*types.Log{},
	}
	aerchan <- aer
	close(aerchan)
	b := mpt.MapToMPTBatch(cache.Store.GetStorageChanges())
	mpt, sr, err := bc.stateRoot.AddMPTBatch(block.Index, b, cache.Store)
	if err != nil {
		// Release goroutines, don't care about errors, we already have one.
		<-aerdone
		// Here MPT can be left in a half-applied state.
		// However if this error occurs, this is a bug somewhere in code
		// because changes applied are the ones from HALTed transactions.

		return fmt.Errorf("error while trying to apply MPT changes: %s", err.Error())
	}

	if bc.config.SaveStorageBatch {
		bc.lastBatch = cache.GetBatch()
	}
	// Every persist cycle we also compact our in-memory MPT. It's flushed
	// already in AddMPTBatch, so collapsing it is safe.
	persistedHeight := atomic.LoadUint32(&bc.persistedHeight)
	if persistedHeight == block.Index-1 {
		// 10 is good and roughly estimated to fit remaining trie into 1M of memory.
		mpt.Collapse(10)
	}

	aererr := <-aerdone
	if aererr != nil {
		bc.log.Debug("receipt save error", zap.Error(aererr))
		return aererr
	}

	bc.lock.Lock()
	_, err = aerCache.Persist()
	if err != nil {
		bc.lock.Unlock()
		return err
	}
	_, err = cache.Persist()
	if err != nil {
		bc.lock.Unlock()
		return err
	}

	mpt.Store = bc.dao.Store
	bc.stateRoot.UpdateCurrentLocal(mpt, sr)
	bc.topBlock.Store(block)
	bc.topBlockAer.Store(aer)
	atomic.StoreUint32(&bc.blockHeight, block.Index)
	bc.memPool.RemoveStale(func(tx *transaction.Transaction) bool { return bc.IsTxStillRelevant(tx, txpool, false) }, bc)
	for _, f := range bc.postBlock {
		f(bc.IsTxStillRelevant, txpool, block)
	}
	if err := bc.updateExtensibleWhitelist(block.Index); err != nil {
		bc.lock.Unlock()
		return err
	}
	bc.lock.Unlock()

	updateBlockHeightMetric(block.Index)
	// Genesis block is stored when Blockchain is not yet running, so there
	// is no one to read this event. And it doesn't make much sense as event
	// anyway.
	if block.Index != 0 {
		bc.events <- bcEvent{block, appExecResults}
	}
	return nil
}

func (bc *Blockchain) onPersist(d *dao.Simple, b *block.Block) error {
	return bc.contracts.OnPersist(d, b)
}

func (bc *Blockchain) postPersist(d *dao.Simple, b *block.Block) error {
	err := bc.contracts.PostPersist(d, b)
	if err != nil {
		return err
	}
	nodes, index, err := bc.contracts.Designate.GetDesignatedByRole(d, noderoles.StateValidator, b.Index)
	if err != nil {
		return err
	}
	bc.stateRoot.UpdateStateValidators(index, nodes)
	return nil
}

func (bc *Blockchain) updateExtensibleWhitelist(height uint32) error {
	stateVals, _, err := bc.contracts.Designate.GetDesignatedByRole(bc.dao, noderoles.StateValidator, height+1)
	if err != nil {
		return err
	}
	validators, err := bc.contracts.Designate.GetValidators(bc.dao, height+1)
	if err != nil {
		return err
	}
	newList := []common.Address{}
	bc.updateExtensibleList(&newList, validators)
	bc.updateExtensibleList(&newList, stateVals)

	sort.Slice(newList, func(i, j int) bool {
		return newList[i].Hash().Big().Cmp(newList[j].Hash().Big()) < 0
	})
	bc.extensible.Store(newList)
	return nil
}

func (bc *Blockchain) updateExtensibleList(s *[]common.Address, pubs keys.PublicKeys) {
	for _, pub := range pubs {
		*s = append(*s, pub.Address())
	}
}

// IsExtensibleAllowed determines if script hash is allowed to send extensible payloads.
func (bc *Blockchain) IsExtensibleAllowed(u common.Address) bool {
	us := bc.extensible.Load().([]common.Address)
	for _, addr := range us {
		if addr == u {
			return true
		}
	}
	return false
}

// GetUtilityTokenBalance returns utility token (GAS) balance for the acc.
func (bc *Blockchain) GetUtilityTokenBalance(acc common.Address) *big.Int {
	bs := bc.contracts.GAS.GetBalance(bc.dao, acc)
	if bs == nil {
		return big.NewInt(0)
	}
	return bs
}

// LastBatch returns last persisted storage batch.
func (bc *Blockchain) LastBatch() *storage.MemBatch {
	return bc.lastBatch
}

// persist flushes current in-memory Store contents to the persistent storage.
func (bc *Blockchain) persist(isSync bool) (time.Duration, error) {
	var (
		start     = time.Now()
		duration  time.Duration
		persisted int
		err       error
	)

	if isSync {
		persisted, err = bc.dao.PersistSync()
	} else {
		persisted, err = bc.dao.Persist()
	}
	if err != nil {
		return 0, err
	}
	if persisted > 0 {
		bHeight, err := bc.persistent.GetCurrentBlockHeight()
		if err != nil {
			return 0, err
		}
		oldHeight := atomic.SwapUint32(&bc.persistedHeight, bHeight)
		diff := bHeight - oldHeight

		storedHeaderHeight, _, err := bc.persistent.GetCurrentHeaderHeight()
		if err != nil {
			return 0, err
		}
		duration = time.Since(start)
		bc.log.Info("persisted to disk",
			zap.Uint32("blocks", diff),
			zap.Int("keys", persisted),
			zap.Uint32("headerHeight", storedHeaderHeight),
			zap.Uint32("blockHeight", bHeight),
			zap.Duration("took", duration))

		// update monitoring metrics.
		updatePersistedHeightMetric(bHeight)
	}

	return duration, nil
}

// GetTransaction returns a TX and its height by the given hash. The height is MaxUint32 if tx is in the mempool.
func (bc *Blockchain) GetTransaction(hash common.Hash) (*transaction.Transaction, *types.Receipt, error) {
	if tx, ok := bc.memPool.TryGetValue(hash); ok {
		return tx, nil, nil // the height is not actually defined for memPool transaction.
	}
	return bc.dao.GetTransaction(hash)
}

// GetStorageItem returns an item from storage.
func (bc *Blockchain) GetStorageItem(hash common.Address, key []byte) state.StorageItem {
	return bc.dao.GetStorageItem(hash, key)
}

// GetStorageItems returns all storage items for a given contract id.
func (bc *Blockchain) GetStorageItems(hash common.Address) ([]state.StorageItemWithKey, error) {
	return bc.dao.GetStorageItems(hash)
}

// GetBlock returns a Block by the given hash.
func (bc *Blockchain) GetBlock(hash common.Hash, full bool) (*block.Block, *types.Receipt, error) {
	topBlock := bc.topBlock.Load()
	if topBlock != nil {
		tb := topBlock.(*block.Block)
		if tb.Hash() == hash {
			return tb, bc.topBlockAer.Load().(*types.Receipt), nil
		}
	}

	block, receipt, err := bc.dao.GetBlock(hash)
	if err != nil {
		return nil, nil, err
	}
	if block.MerkleRoot != (common.Hash{}) && len(block.Transactions) == 0 {
		return nil, nil, errors.New("only header is found")
	}
	if full {
		for _, tx := range block.Transactions {
			stx, _, err := bc.dao.GetTransaction(tx.Hash())
			if err != nil {
				return nil, nil, err
			}
			*tx = *stx
		}
		block.Trimmed = false
	}
	return block, receipt, nil
}

// GetHeader returns data block header identified with the given hash value.
func (bc *Blockchain) GetHeader(hash common.Hash) (*block.Header, error) {
	topBlock := bc.topBlock.Load()
	if topBlock != nil {
		tb := topBlock.(*block.Block)
		if tb.Hash() == hash {
			return &tb.Header, nil
		}
	}
	block, _, err := bc.dao.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	return &block.Header, nil
}

// HasTransaction returns true if the blockchain contains he given
// transaction hash.
func (bc *Blockchain) HasTransaction(hash common.Hash) bool {
	if bc.memPool.ContainsKey(hash) {
		return true
	}
	return bc.dao.HasTransaction(hash) == dao.ErrAlreadyExists
}

// HasBlock returns true if the blockchain contains the given
// block hash.
func (bc *Blockchain) HasBlock(hash common.Hash) bool {
	if header, err := bc.GetHeader(hash); err == nil {
		return header.Index <= bc.BlockHeight()
	}
	return false
}

// CurrentBlockHash returns the highest processed block hash.
func (bc *Blockchain) CurrentBlockHash() common.Hash {
	topBlock := bc.topBlock.Load()
	if topBlock != nil {
		tb := topBlock.(*block.Block)
		return tb.Hash()
	}
	return bc.GetHeaderHash(int(bc.BlockHeight()))
}

// CurrentHeaderHash returns the hash of the latest known header.
func (bc *Blockchain) CurrentHeaderHash() common.Hash {
	bc.headerHashesLock.RLock()
	hash := bc.headerHashes[len(bc.headerHashes)-1]
	bc.headerHashesLock.RUnlock()
	return hash
}

// GetHeaderHash returns hash of the header/block with specified index, if
// Blockchain doesn't have a hash for this height, zero Uint256 value is returned.
func (bc *Blockchain) GetHeaderHash(i int) common.Hash {
	bc.headerHashesLock.RLock()
	defer bc.headerHashesLock.RUnlock()

	hashesLen := len(bc.headerHashes)
	if hashesLen <= i {
		return common.Hash{}
	}
	return bc.headerHashes[i]
}

// BlockHeight returns the height/index of the highest block.
func (bc *Blockchain) BlockHeight() uint32 {
	return atomic.LoadUint32(&bc.blockHeight)
}

// HeaderHeight returns the index/height of the highest header.
func (bc *Blockchain) HeaderHeight() uint32 {
	bc.headerHashesLock.RLock()
	n := len(bc.headerHashes)
	bc.headerHashesLock.RUnlock()
	return uint32(n - 1)
}

// GetContractState returns contract by its script hash.
func (bc *Blockchain) GetContractState(hash common.Address) *state.Contract {
	contract := bc.contracts.Management.GetContract(bc.dao, hash)
	if contract == nil {
		bc.log.Warn("failed to get contract state")
	}
	return contract
}

// GetNativeContractScriptHash returns native contract script hash by its name.
func (bc *Blockchain) GetNativeContractScriptHash(name string) (common.Address, error) {
	c := bc.contracts.ByName(name)
	if c != nil {
		return c.Address, nil
	}
	return common.Address{}, errors.New("unknown native contract")
}

// GetNatives returns list of native contracts.
func (bc *Blockchain) GetNatives() []state.NativeContract {
	return bc.contracts.Contracts
}

// GetConfig returns the config stored in the blockchain.
func (bc *Blockchain) GetConfig() config.ProtocolConfiguration {
	return bc.config
}

// SubscribeForBlocks adds given channel to new block event broadcasting, so when
// there is a new block added to the chain you'll receive it via this channel.
// Make sure it's read from regularly as not reading these events might affect
// other Blockchain functions.
func (bc *Blockchain) SubscribeForBlocks(ch chan<- *block.Block) {
	bc.subCh <- ch
}

// SubscribeForTransactions adds given channel to new transaction event
// broadcasting, so when there is a new transaction added to the chain (in a
// block) you'll receive it via this channel. Make sure it's read from regularly
// as not reading these events might affect other Blockchain functions.
func (bc *Blockchain) SubscribeForTransactions(ch chan<- *transaction.Transaction) {
	bc.subCh <- ch
}

// SubscribeForNotifications adds given channel to new notifications event
// broadcasting, so when an in-block transaction execution generates a
// notification you'll receive it via this channel. Only notifications from
// successful transactions are broadcasted, if you're interested in failed
// transactions use SubscribeForExecutions instead. Make sure this channel is
// read from regularly as not reading these events might affect other Blockchain
// functions.
func (bc *Blockchain) SubscribeForNotifications(ch chan<- *types.Log) {
	bc.subCh <- ch
}

// SubscribeForExecutions adds given channel to new transaction execution event
// broadcasting, so when an in-block transaction execution happens you'll receive
// the result of it via this channel. Make sure it's read from regularly as not
// reading these events might affect other Blockchain functions.
func (bc *Blockchain) SubscribeForExecutions(ch chan<- *types.Receipt) {
	bc.subCh <- ch
}

// UnsubscribeFromBlocks unsubscribes given channel from new block notifications,
// you can close it afterwards. Passing non-subscribed channel is a no-op.
func (bc *Blockchain) UnsubscribeFromBlocks(ch chan<- *block.Block) {
	bc.unsubCh <- ch
}

// UnsubscribeFromTransactions unsubscribes given channel from new transaction
// notifications, you can close it afterwards. Passing non-subscribed channel is
// a no-op.
func (bc *Blockchain) UnsubscribeFromTransactions(ch chan<- *transaction.Transaction) {
	bc.unsubCh <- ch
}

// UnsubscribeFromNotifications unsubscribes given channel from new
// execution-generated notifications, you can close it afterwards. Passing
// non-subscribed channel is a no-op.
func (bc *Blockchain) UnsubscribeFromNotifications(ch chan<- *types.Log) {
	bc.unsubCh <- ch
}

// UnsubscribeFromExecutions unsubscribes given channel from new execution
// notifications, you can close it afterwards. Passing non-subscribed channel is
// a no-op.
func (bc *Blockchain) UnsubscribeFromExecutions(ch chan<- *types.Receipt) {
	bc.unsubCh <- ch
}

// FeePerByte returns transaction network fee per byte.
func (bc *Blockchain) FeePerByte() uint64 {
	return bc.contracts.Policy.GetFeePerByte(bc.dao)
}

// GetMemPool returns the memory pool of the blockchain.
func (bc *Blockchain) GetMemPool() *mempool.Pool {
	return bc.memPool
}

// ApplyPolicyToTxSet applies configured policies to given transaction set. It
// expects slice to be ordered by fee and returns a subslice of it.
func (bc *Blockchain) ApplyPolicyToTxSet(txes []*transaction.Transaction) []*transaction.Transaction {
	maxTx := bc.config.MaxTransactionsPerBlock
	if maxTx != 0 && len(txes) > int(maxTx) {
		txes = txes[:maxTx]
	}
	validators, _ := bc.contracts.Designate.GetValidators(bc.dao, bc.BlockHeight()+1)
	maxBlockSize := bc.config.MaxBlockSize
	maxBlockSysFee := bc.config.MaxBlockGas
	oldVC := bc.knownValidatorsCount.Load()
	defaultWitness := bc.defaultBlockWitness.Load()
	curVC := len(validators)
	if oldVC == nil || oldVC != curVC {
		m := consensus.GetDefaultHonestNodeCount(curVC)
		verification, _ := validators.CreateDefaultMultiSigRedeemScript()
		defaultWitness = transaction.Witness{
			InvocationScript:   make([]byte, 65*m+1),
			VerificationScript: verification,
		}
		bc.knownValidatorsCount.Store(curVC)
		bc.defaultBlockWitness.Store(defaultWitness)
	}
	var (
		b           = &block.Block{Header: block.Header{Witness: defaultWitness.(transaction.Witness)}}
		blockSize   = uint32(b.GetExpectedBlockSizeWithoutTransactions(len(txes)))
		blockSysFee uint64
	)
	for i, tx := range txes {
		blockSize += uint32(tx.Size())
		blockSysFee += tx.Gas()
		if blockSize > maxBlockSize || blockSysFee > maxBlockSysFee {
			txes = txes[:i]
			break
		}
	}
	return txes
}

// Various errors that could be returns upon header verification.
var (
	ErrHdrHashMismatch     = errors.New("previous header hash doesn't match")
	ErrHdrIndexMismatch    = errors.New("previous header index doesn't match")
	ErrHdrInvalidTimestamp = errors.New("block is not newer than the previous one")
	ErrHdrStateRootSetting = errors.New("state root setting mismatch")
	ErrHdrInvalidStateRoot = errors.New("state root for previous block is invalid")
)

func (bc *Blockchain) verifyHeader(currHeader, prevHeader *block.Header) error {
	if prevHeader.Hash() != currHeader.PrevHash {
		return ErrHdrHashMismatch
	}
	if prevHeader.Index+1 != currHeader.Index {
		return ErrHdrIndexMismatch
	}
	if prevHeader.Timestamp >= currHeader.Timestamp {
		return ErrHdrInvalidTimestamp
	}
	return bc.verifyHeaderWitness(currHeader, prevHeader)
}

// Various errors that could be returned upon verification.
var (
	ErrTxExpired         = errors.New("transaction has expired")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrTxSmallNetworkFee = errors.New("too small network fee")
	ErrTxTooBig          = errors.New("too big transaction")
	ErrMemPoolConflict   = errors.New("invalid transaction due to conflicts with the memory pool")
	ErrInvalidScript     = errors.New("invalid script")
	ErrInvalidAttribute  = errors.New("invalid attribute")
)

// verifyAndPoolTx verifies whether a transaction is bonafide or not and tries
// to add it to the mempool given.
func (bc *Blockchain) verifyAndPoolTx(t *transaction.Transaction, pool *mempool.Pool, feer mempool.Feer, data ...interface{}) error {
	err := t.IsValid()
	if err != nil {
		return err
	}
	if t.Gas() > bc.config.MaxBlockGas {
		return fmt.Errorf("gas exceeds block gas limit, limit: %d, actual: %d", bc.config.MaxBlockGas, t.Gas())
	}
	if t.GasPrice().Cmp(bc.GetGasPrice()) < 0 {
		return fmt.Errorf("gas price too low, expect %s, actual %s", bc.GetGasPrice(), t.GasPrice())
	}
	size := t.Size()
	if size > transaction.MaxTransactionSize {
		return fmt.Errorf("%w: (%d > MaxTransactionSize %d)", ErrTxTooBig, size, transaction.MaxTransactionSize)
	}

	needNetworkFee := transaction.CalculateNetworkFee(t, bc.FeePerByte())
	if t.Gas() < needNetworkFee {
		return fmt.Errorf("%w: net fee is %v, need %v", ErrTxSmallNetworkFee, t.Gas(), needNetworkFee)
	}
	// check that current tx wasn't included in the conflicts attributes of some other transaction which is already in the chain
	if err := bc.dao.HasTransaction(t.Hash()); err != nil {
		switch {
		case errors.Is(err, dao.ErrAlreadyExists):
			return fmt.Errorf("blockchain: %w", ErrAlreadyExists)
		case errors.Is(err, dao.ErrHasConflicts):
			return fmt.Errorf("blockchain: %w", ErrHasConflicts)
		default:
			return err
		}
	}

	// From need to recover from signature, so place this infront of nonce and policy check
	err = t.Verify(bc.config.ChainID)
	if err != nil {
		return err
	}

	from := t.From()
	nonce := bc.GetNonce(from)

	if t.Nonce() != nonce {
		return fmt.Errorf("invalid nonce, addr=%s, nonce=%d, expect=%d", t.From(), t.Nonce(), nonce)
	}

	if err := bc.PolicyCheck(t); err != nil {
		return fmt.Errorf("%w: %v", ErrPolicy, err)
	}

	err = pool.Add(t, feer, data...)
	if err != nil {
		switch {
		case errors.Is(err, mempool.ErrConflict):
			return ErrMemPoolConflict
		case errors.Is(err, mempool.ErrDup):
			return fmt.Errorf("mempool: %w", ErrAlreadyExists)
		case errors.Is(err, mempool.ErrInsufficientFunds):
			return ErrInsufficientFunds
		case errors.Is(err, mempool.ErrOOM):
			return ErrOOM
		case errors.Is(err, mempool.ErrConflictsNonce):
			return fmt.Errorf("mempool: %w: %s", ErrHasConflicts, err)
		default:
			return err
		}
	}

	return nil
}

func (bc *Blockchain) PolicyCheck(t *transaction.Transaction) error {
	if bc.contracts.Policy.IsBlocked(bc.dao, t.From()) {
		return native.ErrAccountBlocked
	}
	if t.To() != nil &&
		bc.contracts.Management.GetContract(bc.dao, *t.To()) != nil &&
		bc.contracts.Policy.IsBlocked(bc.dao, *t.To()) {
		return native.ErrContractBlocked
	}
	return nil
}

// IsTxStillRelevant is a callback for mempool transaction filtering after the
// new block addition. It returns false for transactions added by the new block
// (passed via txpool) and does witness reverification for non-standard
// contracts. It operates under the assumption that full transaction verification
// was already done so we don't need to check basic things like size, input/output
// correctness, presence in blocks before the new one, etc.
func (bc *Blockchain) IsTxStillRelevant(t *transaction.Transaction, txpool *mempool.Pool, isPartialTx bool) bool {
	var recheckWitness bool
	if txpool == nil {
		if bc.dao.HasTransaction(t.Hash()) != nil {
			return false
		}
	} else if txpool.HasConflicts(t, bc) {
		return false
	}
	if bc.GetNonce(t.From()) != t.Nonce() {
		return false
	}
	if recheckWitness {
		return t.Verify(bc.config.ChainID) == nil
	}
	return true
}

// VerifyTx verifies whether transaction is bonafide or not relative to the
// current blockchain state. Note that this verification is completely isolated
// from the main node's mempool.
func (bc *Blockchain) VerifyTx(t *transaction.Transaction) error {
	var mp = mempool.New(1, 0, false)
	bc.lock.RLock()
	defer bc.lock.RUnlock()
	return bc.verifyAndPoolTx(t, mp, bc)
}

// PoolTx verifies and tries to add given transaction into the mempool. If not
// given, the default mempool is used. Passing multiple pools is not supported.
func (bc *Blockchain) PoolTx(t *transaction.Transaction, pools ...*mempool.Pool) error {
	var pool = bc.memPool

	bc.lock.RLock()
	defer bc.lock.RUnlock()
	// Programmer error.
	if len(pools) > 1 {
		panic("too many pools given")
	}
	if len(pools) == 1 {
		pool = pools[0]
	}
	return bc.verifyAndPoolTx(t, pool, bc)
}

// PoolTxWithData verifies and tries to add given transaction with additional data into the mempool.
func (bc *Blockchain) PoolTxWithData(t *transaction.Transaction, data interface{}, mp *mempool.Pool, feer mempool.Feer, verificationFunction func(tx *transaction.Transaction, data interface{}) error) error {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	if verificationFunction != nil {
		err := verificationFunction(t, data)
		if err != nil {
			return err
		}
	}
	return bc.verifyAndPoolTx(t, mp, feer, data)
}

func (bc *Blockchain) GetConsensusAddress() (common.Address, error) {
	return bc.contracts.Designate.GetConsensusAddress(bc.dao, bc.BlockHeight()+1)
}

// GetValidators returns current validators.
func (bc *Blockchain) GetValidators(index uint32) ([]*keys.PublicKey, error) {
	return bc.contracts.Designate.GetValidators(bc.dao, index)
}

func (bc *Blockchain) GetCurrentValidators() ([]*keys.PublicKey, error) {
	return bc.contracts.Designate.GetValidators(bc.dao, bc.BlockHeight()+1)
}

func (bc *Blockchain) IsBlocked(address common.Address) bool {
	return bc.contracts.Policy.IsBlocked(bc.dao, address)
}

// GetTestVM returns an interop context with VM set up for a test run.
func (bc *Blockchain) GetTestVM(tx *transaction.Transaction, b *block.Block, tracer vm.EVMLogger) (*interop.Context, error) {
	cache := bc.dao.GetPrivate()
	sdb := statedb.NewStateDB(cache, bc)
	sdb.PrepareAccessList(tx.From(), tx.To(), evm.PrecompiledAddressesBerlin, tx.AccessList())
	return interop.NewContext(b, tx, sdb, bc, tracer)
}

// Various witness verification errors.
var (
	ErrWitnessHashMismatch = errors.New("witness address mismatch")
	ErrVerificationFailed  = errors.New("signature check failed")
	ErrInvalidInvocation   = errors.New("invalid invocation script")
	ErrInvalidSignature    = fmt.Errorf("%w: invalid signature", ErrVerificationFailed)
	ErrInvalidVerification = errors.New("invalid verification script")
)

// VerifyWitness checks that w is a correct witness for c signed by h. It returns
// the amount of GAS consumed during verification and an error.
func (bc *Blockchain) VerifyWitness(h common.Address, c hash.Hashable, w *transaction.Witness) error {
	if h != w.Address() {
		return ErrWitnessHashMismatch
	}
	return w.VerifyHashable(bc.config.ChainID, c)
}

// verifyHeaderWitness is a block-specific implementation of VerifyWitnesses logic.
func (bc *Blockchain) verifyHeaderWitness(currHeader, prevHeader *block.Header) error {
	var consensus common.Address
	if prevHeader == nil && currHeader.PrevHash == (common.Hash{}) {
		consensus = currHeader.Witness.Address()
	} else {
		consensus = prevHeader.NextConsensus
	}
	return bc.VerifyWitness(consensus, currHeader, &currHeader.Witness)
}

// UtilityTokenHash returns the utility token (GAS) native contract hash.
func (bc *Blockchain) UtilityTokenAddress() common.Address {
	return bc.contracts.GAS.Address
}

// ManagementContractHash returns management contract's hash.
func (bc *Blockchain) ManagementContractAddress() common.Address {
	return bc.contracts.Management.Address
}

// RegisterPostBlock appends provided function to the list of functions which should be run after new block
// is stored.
func (bc *Blockchain) RegisterPostBlock(f func(func(*transaction.Transaction, *mempool.Pool, bool) bool, *mempool.Pool, *block.Block)) {
	bc.postBlock = append(bc.postBlock, f)
}

func (bc *Blockchain) GetFeePerByte() uint64 {
	if bc.BlockHeight() == 0 {
		return native.DefaultFeePerByte
	}
	return bc.contracts.Policy.GetFeePerByte(bc.dao)
}

func (bc *Blockchain) GetGasPrice() *big.Int {
	if bc.BlockHeight() == 0 {
		return big.NewInt(int64(native.DefaultGasPrice))
	}
	return bc.contracts.Policy.GetGasPrice(bc.dao)
}

func (bc *Blockchain) Contracts() *native.Contracts {
	return &bc.contracts
}

func (bc *Blockchain) GetNonce(addr common.Address) uint64 {
	return bc.contracts.Ledger.GetNonce(bc.dao, addr)
}

func (bc *Blockchain) GetLogs(filter *filters.LogFilter) ([]*types.Log, error) {
	blockhashes := []common.Hash{}
	if filter.Blockhash != (common.Hash{}) {
		blockhashes = append(blockhashes, filter.Blockhash)
	} else {
		for i := filter.FromBlock; i <= filter.ToBlock; i++ {
			hash := bc.GetHeaderHash(int(i))
			if hash == (common.Hash{}) {
				break
			}
			blockhashes = append(blockhashes, hash)
		}
	}
	if len(blockhashes) == 0 {
		return nil, nil
	}
	logs := []*types.Log{}
	for _, hash := range blockhashes {
		block, _, err := bc.GetBlock(hash, false)
		if err != nil {
			return nil, err
		}
		for _, tx := range block.Transactions {
			_, appExec, err := bc.GetTransaction(tx.Hash())
			println("=========", appExec.Logs)
			if err != nil {
				return nil, err
			}
			if appExec != nil {
				for _, l := range appExec.Logs {
					if filter.Match(l) {
						logs = append(logs, l)
					}
				}
			}
		}
	}
	return logs, nil
}

func (bc *Blockchain) GetMinted(id int64) (common.Hash, error) {
	return bc.contracts.Bridge.GetMinted(bc.dao, id)
}
