package stateroot

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/mpt"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type (
	// VerifierFunc is a function that allows to check witness of account
	// for Hashable item with GAS limit.
	VerifierFunc func(common.Address, hash.Hashable, *transaction.Witness) error
	// Module represents module for local processing of state roots.
	Module struct {
		Store    *storage.MemCachedStore
		chainId  uint64
		mode     mpt.TrieMode
		mpt      *mpt.Trie
		verifier VerifierFunc
		log      *zap.Logger

		currentLocal    atomic.Value
		localHeight     atomic.Uint32
		validatedHeight atomic.Uint32

		mtx  sync.RWMutex
		keys []keyCache

		updateValidatorsCb func(height uint32, publicKeys keys.PublicKeys)
	}

	keyCache struct {
		height           uint32
		validatorsKeys   keys.PublicKeys
		validatorsHash   common.Address
		validatorsScript []byte
	}
)

// NewModule returns new instance of stateroot module.
func NewModule(cfg config.ProtocolConfiguration, verif VerifierFunc, log *zap.Logger, s *storage.MemCachedStore) *Module {
	var mode mpt.TrieMode
	if cfg.KeepOnlyLatestState {
		mode |= mpt.ModeLatest
	}
	if cfg.RemoveUntraceableBlocks {
		mode |= mpt.ModeGC
	}
	return &Module{
		chainId:  cfg.ChainID,
		mode:     mode,
		verifier: verif,
		log:      log,
		Store:    s,
	}
}

// GetState returns value at the specified key fom the MPT with the specified root.
func (s *Module) GetState(root common.Hash, key []byte) ([]byte, error) {
	// Allow accessing old values, it's RO thing.
	tr := mpt.NewTrie(mpt.NewHashNode(root), s.mode&^mpt.ModeGCFlag, storage.NewMemCachedStore(s.Store))
	return tr.Get(key)
}

// FindStates returns set of key-value pairs with key matching the prefix starting
// from the `prefix`+`start` path from MPT trie with the specified root. `max` is
// the maximum number of elements to be returned. If nil `start` specified, then
// item with key equals to prefix is included into result; if empty `start` specified,
// then item with key equals to prefix is not included into result.
func (s *Module) FindStates(root common.Hash, prefix, start []byte, max int) ([]storage.KeyValue, error) {
	// Allow accessing old values, it's RO thing.
	tr := mpt.NewTrie(mpt.NewHashNode(root), s.mode&^mpt.ModeGCFlag, storage.NewMemCachedStore(s.Store))
	return tr.Find(prefix, start, max)
}

// GetStateProof returns proof of having key in the MPT with the specified root.
func (s *Module) GetStateProof(root common.Hash, key []byte) ([][]byte, error) {
	// Allow accessing old values, it's RO thing.
	tr := mpt.NewTrie(mpt.NewHashNode(root), s.mode&^mpt.ModeGCFlag, storage.NewMemCachedStore(s.Store))
	return tr.GetProof(key)
}

// GetStateRoot returns state root for a given height.
func (s *Module) GetStateRoot(height uint32) (*state.MPTRoot, error) {
	return s.getStateRoot(makeStateRootKey(height))
}

// CurrentLocalStateRoot returns hash of the local state root.
func (s *Module) CurrentLocalStateRoot() common.Hash {
	return s.currentLocal.Load().(common.Hash)
}

// CurrentLocalHeight returns height of the local state root.
func (s *Module) CurrentLocalHeight() uint32 {
	return s.localHeight.Load()
}

// CurrentValidatedHeight returns current state root validated height.
func (s *Module) CurrentValidatedHeight() uint32 {
	return s.validatedHeight.Load()
}

// Init initializes state root module at the given height.
func (s *Module) Init(height uint32) error {
	data, err := s.Store.Get([]byte{byte(storage.DataMPTAux), prefixValidated})
	if err == nil {
		s.validatedHeight.Store(binary.LittleEndian.Uint32(data))
	}

	if height == 0 {
		s.mpt = mpt.NewTrie(nil, s.mode, s.Store)
		s.currentLocal.Store(common.Hash{})
		return nil
	}
	r, err := s.getStateRoot(makeStateRootKey(height))
	if err != nil {
		return err
	}
	s.currentLocal.Store(r.Root)
	s.localHeight.Store(r.Index)
	s.mpt = mpt.NewTrie(mpt.NewHashNode(r.Root), s.mode, s.Store)
	return nil
}

// CleanStorage removes all MPT-related data from the storage (MPT nodes, validated stateroots)
// except local stateroot for the current height and GC flag. This method is aimed to clean
// outdated MPT data before state sync process can be started.
// Note: this method is aimed to be called for genesis block only, an error is returned otherwice.
func (s *Module) CleanStorage() error {
	if s.localHeight.Load() != 0 {
		return fmt.Errorf("can't clean MPT data for non-genesis block: expected local stateroot height 0, got %d", s.localHeight.Load())
	}
	b := storage.NewMemCachedStore(s.Store)
	s.Store.Seek(storage.SeekRange{Prefix: []byte{byte(storage.DataMPT)}}, func(k, _ []byte) bool {
		// #1468, but don't need to copy here, because it is done by Store.
		b.Delete(k)
		return true
	})
	_, err := b.Persist()
	if err != nil {
		return fmt.Errorf("failed to remove outdated MPT-reated items: %w", err)
	}
	return nil
}

// JumpToState performs jump to the state specified by given stateroot index.
func (s *Module) JumpToState(sr *state.MPTRoot) {
	s.addLocalStateRoot(s.Store, sr)

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	s.Store.Put([]byte{byte(storage.DataMPTAux), prefixValidated}, data)
	s.validatedHeight.Store(sr.Index)

	s.currentLocal.Store(sr.Root)
	s.localHeight.Store(sr.Index)
	s.mpt = mpt.NewTrie(mpt.NewHashNode(sr.Root), s.mode, s.Store)
}

// GC performs garbage collection.
func (s *Module) GC(index uint32, store storage.Store) time.Duration {
	if !s.mode.GC() {
		panic("stateroot: GC invoked, but not enabled")
	}
	var removed int
	var stored int64
	s.log.Info("starting MPT garbage collection", zap.Uint32("index", index))
	start := time.Now()
	err := store.SeekGC(storage.SeekRange{
		Prefix: []byte{byte(storage.DataMPT)},
	}, func(k, v []byte) bool {
		stored++
		if !mpt.IsActiveValue(v) {
			h := binary.LittleEndian.Uint32(v[len(v)-4:])
			if h <= index {
				removed++
				stored--
				return false
			}
		}
		return true
	})
	dur := time.Since(start)
	if err != nil {
		s.log.Error("failed to flush MPT GC changeset", zap.Duration("time", dur), zap.Error(err))
	} else {
		s.log.Info("finished MPT garbage collection",
			zap.Int("removed", removed),
			zap.Int64("kept", stored),
			zap.Duration("time", dur))
	}
	return dur
}

// AddMPTBatch updates using provided batch.
func (s *Module) AddMPTBatch(index uint32, b mpt.Batch, cache *storage.MemCachedStore) (*mpt.Trie, *state.MPTRoot, error) {
	mpt := *s.mpt
	mpt.Store = cache
	if _, err := mpt.PutBatch(b); err != nil {
		return nil, nil, err
	}
	mpt.Flush(index)
	sr := &state.MPTRoot{
		Index: index,
		Root:  mpt.StateRoot(),
	}
	s.addLocalStateRoot(cache, sr)
	return &mpt, sr, nil
}

// UpdateCurrentLocal updates local caches using provided state root.
func (s *Module) UpdateCurrentLocal(mpt *mpt.Trie, sr *state.MPTRoot) {
	s.mpt = mpt
	s.currentLocal.Store(sr.Root)
	s.localHeight.Store(sr.Index)
}

// VerifyStateRoot checks if state root is valid.
func (s *Module) VerifyStateRoot(r *state.MPTRoot) error {
	_, err := s.getStateRoot(makeStateRootKey(r.Index - 1))
	if err != nil {
		return errors.New("can't get previous state root")
	}
	if len(r.Witness.VerificationScript) == 0 {
		return errors.New("no witness")
	}
	return s.verifyWitness(r)
}

const maxVerificationGAS = 2_00000000

// verifyWitness verifies state root witness.
func (s *Module) verifyWitness(r *state.MPTRoot) error {
	s.mtx.Lock()
	h := s.getKeyCacheForHeight(r.Index).validatorsHash
	s.mtx.Unlock()
	return s.verifier(h, r, &r.Witness)
}
