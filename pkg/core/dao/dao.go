package dao

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/storage"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/util/slice"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// HasTransaction errors.
var (
	// ErrAlreadyExists is returned when transaction exists in dao.
	ErrAlreadyExists = errors.New("transaction already exists")
	// ErrHasConflicts is returned when transaction is in the list of conflicting
	// transactions which are already in dao.
	ErrHasConflicts = errors.New("transaction has conflicts")
)

// Simple is memCached wrapper around DB, simple DAO implementation.
type Simple struct {
	Version Version
	Store   *storage.MemCachedStore
	private bool
	keyBuf  []byte
	dataBuf *io.BufBinWriter
}

// NewSimple creates new simple dao using provided backend store.
func NewSimple(backend storage.Store) *Simple {
	st := storage.NewMemCachedStore(backend)
	return newSimple(st)
}

func newSimple(st *storage.MemCachedStore) *Simple {
	return &Simple{
		Version: Version{
			StoragePrefix: storage.STStorage,
		},
		Store: st,
	}
}

// GetBatch returns currently accumulated DB changeset.
func (dao *Simple) GetBatch() *storage.MemBatch {
	return dao.Store.GetBatch()
}

// GetWrapped returns new DAO instance with another layer of wrapped
// MemCachedStore around the current DAO Store.
func (dao *Simple) GetWrapped() *Simple {
	d := NewSimple(dao.Store)
	d.Version = dao.Version
	return d
}

// GetPrivate returns new DAO instance with another layer of private
// MemCachedStore around the current DAO Store.
func (dao *Simple) GetPrivate() *Simple {
	d := &Simple{}
	*d = *dao                                             // Inherit everything...
	d.Store = storage.NewPrivateMemCachedStore(dao.Store) // except storage, wrap another layer.
	d.private = true
	return d
}

// GetAndDecode performs get operation and decoding with serializable structures.
func (dao *Simple) GetAndDecode(entity io.Serializable, key []byte) error {
	entityBytes, err := dao.Store.Get(key)
	if err != nil {
		return err
	}
	reader := io.NewBinReaderFromBuf(entityBytes)
	entity.DecodeBinary(reader)
	return reader.Err
}

// -- start block

func (dao *Simple) makeBlockKey(hash common.Hash) []byte {
	key := dao.getKeyBuf(1 + common.HashLength)
	key[0] = byte(storage.DataBlock)
	copy(key[1:], hash.Bytes())
	return key
}

// GetBlock returns Block by the given hash if it exists in the store.
func (dao *Simple) GetBlock(hash common.Hash) (*block.Block, *types.Receipt, error) {
	return dao.getBlock(dao.makeBlockKey(hash))
}

func (dao *Simple) getBlock(key []byte) (*block.Block, *types.Receipt, error) {
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, nil, err
	}
	r := io.NewBinReaderFromBuf(b)
	bb := r.ReadVarBytes()
	rb := r.ReadVarBytes()
	r = io.NewBinReaderFromBuf(bb)
	block, err := block.NewTrimmedFromReader(r)
	if err != nil {
		return nil, nil, err
	}
	receipt := new(types.Receipt)
	err = json.Unmarshal(rb, receipt)
	if err != nil {
		return nil, nil, err
	}
	return block, receipt, nil
}

func (dao *Simple) StoreAsBlock(block *block.Block, receipt *types.Receipt) error {
	key := dao.makeBlockKey(block.Hash())

	buf := io.NewBufBinWriter()
	block.EncodeTrimmed(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	bb := buf.Bytes()
	rb, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	buf = io.NewBufBinWriter()
	buf.WriteVarBytes(bb)
	buf.WriteVarBytes(rb)
	b := buf.Bytes()
	dao.Store.Put(key, b)
	return nil
}

// DeleteBlock removes block from dao. It's not atomic, so make sure you're
// using private MemCached instance here.
func (dao *Simple) DeleteBlock(h common.Hash) error {
	key := dao.makeBlockKey(h)
	b, _, err := dao.getBlock(key)
	if err != nil {
		return err
	}
	for _, tx := range b.Transactions {
		copy(key[1:], tx.Hash().Bytes())
		dao.Store.Delete(key)
	}
	return nil
}

// -- end block

// -- start notification event.

func (dao *Simple) makeTxKey(hash common.Hash) []byte {
	key := dao.getKeyBuf(1 + common.HashLength)
	key[0] = byte(storage.DataTx)
	copy(key[1:], hash.Bytes())
	return key
}

func (dao *Simple) StoreAsTransaction(tx *transaction.Transaction, aer *types.Receipt) error {
	key := dao.makeTxKey(tx.Hash())
	buf := io.NewBufBinWriter()
	bTx, err := io.ToByteArray(tx)
	if err != nil {
		return err
	}
	buf.WriteVarBytes(bTx)
	b, err := json.Marshal(aer)
	if err != nil {
		return err
	}
	buf.WriteVarBytes(b)
	if buf.Err != nil {
		return buf.Err
	}
	dao.Store.Put(key, buf.Bytes())
	return nil
}

// GetAppExecResults gets application execution results with the specified trigger from the
// given store.
func (dao *Simple) GetReceipt(hash common.Hash) (*types.Receipt, error) {
	key := dao.makeTxKey(hash)
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, err
	}
	if len(b) < 6 {
		return nil, errors.New("bad transaction bytes")
	}
	r := io.NewBinReaderFromBuf(b)
	_ = r.ReadU32LE()
	bTx := r.ReadVarBytes()
	tx := &transaction.Transaction{}
	err = io.FromByteArray(tx, bTx)
	if err != nil {
		return nil, err
	}
	bReceipt := r.ReadVarBytes()
	receipt := &types.Receipt{}
	err = json.Unmarshal(bReceipt, receipt)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

// GetTransaction returns Transaction and its height by the given hash
// if it exists in the store. It does not return dummy transactions.
func (dao *Simple) GetTransaction(hash common.Hash) (*transaction.Transaction, *types.Receipt, error) {
	key := dao.makeTxKey(hash)
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, nil, err
	}
	if len(b) < 6 {
		return nil, nil, errors.New("bad transaction bytes")
	}
	r := io.NewBinReaderFromBuf(b)
	tx := &transaction.Transaction{}
	bTx := r.ReadVarBytes()
	err = io.FromByteArray(tx, bTx)
	if err != nil {
		return nil, nil, err
	}
	bReceipt := r.ReadVarBytes()
	receipt := &types.Receipt{}
	err = json.Unmarshal(bReceipt, receipt)
	if err != nil {
		return nil, nil, err
	}
	return tx, receipt, nil
}

// -- end notification event.

// -- start storage item.

// GetStorageItem returns StorageItem if it exists in the given store.
func (dao *Simple) GetStorageItem(address common.Address, key []byte) state.StorageItem {
	b, err := dao.Store.Get(dao.makeStorageItemKey(address, key))
	if err != nil {
		return nil
	}
	return b
}

// PutStorageItem puts given StorageItem for given id with given
// key into the given store.
func (dao *Simple) PutStorageItem(hash common.Address, key []byte, si state.StorageItem) {
	stKey := dao.makeStorageItemKey(hash, key)
	dao.Store.Put(stKey, si)
}

// DeleteStorageItem drops storage item for the given id with the
// given key from the store.
func (dao *Simple) DeleteStorageItem(hash common.Address, key []byte) {
	stKey := dao.makeStorageItemKey(hash, key)
	dao.Store.Delete(stKey)
}

// GetStorageItems returns all storage items for a given id.
func (dao *Simple) GetStorageItems(hash common.Address) ([]state.StorageItemWithKey, error) {
	return dao.GetStorageItemsWithPrefix(hash, nil)
}

// GetStorageItemsWithPrefix returns all storage items with given id for a
// given scripthash.
func (dao *Simple) GetStorageItemsWithPrefix(hash common.Address, prefix []byte) ([]state.StorageItemWithKey, error) {
	var siArr []state.StorageItemWithKey

	saveToArr := func(k, v []byte) bool {
		// Cut prefix and hash.
		// #1468, but don't need to copy here, because it is done by Store.
		siArr = append(siArr, state.StorageItemWithKey{
			Key:  k,
			Item: state.StorageItem(v),
		})
		return true
	}
	dao.Seek(hash, storage.SeekRange{Prefix: prefix}, saveToArr)
	return siArr, nil
}

// Seek executes f for all storage items matching a given `rng` (matching given prefix and
// starting from the point specified). If key or value is to be used outside of f, they
// may not be copied. Seek continues iterating until false is returned from f.
func (dao *Simple) Seek(hash common.Address, rng storage.SeekRange, f func(k, v []byte) bool) {
	rng.Prefix = slice.Copy(dao.makeStorageItemKey(hash, rng.Prefix)) // f() can use dao too.
	dao.Store.Seek(rng, func(k, v []byte) bool {
		return f(k[len(rng.Prefix):], v)
	})
}

// SeekAsync sends all storage items matching a given `rng` (matching given prefix and
// starting from the point specified) to a channel and returns the channel.
// Resulting keys and values may not be copied.
func (dao *Simple) SeekAsync(ctx context.Context, hash common.Address, rng storage.SeekRange) chan storage.KeyValue {
	rng.Prefix = slice.Copy(dao.makeStorageItemKey(hash, rng.Prefix))
	return dao.Store.SeekAsync(ctx, rng, true)
}

// makeStorageItemKey returns a key used to store StorageItem in the DB.
func (dao *Simple) makeStorageItemKey(hash common.Address, key []byte) []byte {
	// 1 for prefix + 20 for address + len(key) for key
	buf := dao.getKeyBuf(21 + len(key))
	buf[0] = byte(dao.Version.StoragePrefix)
	copy(buf[1:], hash.Bytes())
	copy(buf[21:], key)
	return buf
}

// -- end storage item.

// -- other.

// Version represents current dao version.
type Version struct {
	StoragePrefix       storage.KeyPrefix
	KeepOnlyLatestState bool
	Value               string
}

const (
	keepOnlyLatestStateBit = 1 << iota
)

// FromBytes decodes v from a byte-slice.
func (v *Version) FromBytes(data []byte) error {
	if len(data) == 0 {
		return errors.New("missing version")
	}
	i := 0
	for ; i < len(data) && data[i] != '\x00'; i++ {
	}

	if i == len(data) {
		v.Value = string(data)
		return nil
	}

	if len(data) != i+3 {
		return errors.New("version is invalid")
	}

	v.Value = string(data[:i])
	v.StoragePrefix = storage.KeyPrefix(data[i+1])
	v.KeepOnlyLatestState = data[i+2]&keepOnlyLatestStateBit != 0
	return nil
}

// Bytes encodes v to a byte-slice.
func (v *Version) Bytes() []byte {
	var mask byte
	if v.KeepOnlyLatestState {
		mask |= keepOnlyLatestStateBit
	}
	return append([]byte(v.Value), '\x00', byte(v.StoragePrefix), mask)
}

func (dao *Simple) mkKeyPrefix(k storage.KeyPrefix) []byte {
	b := dao.getKeyBuf(1)
	b[0] = byte(k)
	return b
}

// GetVersion attempts to get the current version stored in the
// underlying store.
func (dao *Simple) GetVersion() (Version, error) {
	var version Version

	data, err := dao.Store.Get(dao.mkKeyPrefix(storage.SYSVersion))
	if err == nil {
		err = version.FromBytes(data)
	}
	return version, err
}

// GetCurrentBlockHeight returns the current block height found in the
// underlying store.
func (dao *Simple) GetCurrentBlockHeight() (uint32, error) {
	b, err := dao.Store.Get(dao.mkKeyPrefix(storage.SYSCurrentBlock))
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[32:36]), nil
}

// GetCurrentHeaderHeight returns the current header height and hash from
// the underlying store.
func (dao *Simple) GetCurrentHeaderHeight() (i uint32, h common.Hash, err error) {
	var b []byte
	b, err = dao.Store.Get(dao.mkKeyPrefix(storage.SYSCurrentHeader))
	if err != nil {
		return
	}
	i = binary.LittleEndian.Uint32(b[32:36])
	h = common.BytesToHash(b[:32])
	return
}

// GetStateSyncPoint returns current state synchronisation point P.
func (dao *Simple) GetStateSyncPoint() (uint32, error) {
	b, err := dao.Store.Get(dao.mkKeyPrefix(storage.SYSStateSyncPoint))
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

// GetHeaderHashes returns a sorted list of header hashes retrieved from
// the given underlying store.
func (dao *Simple) GetHeaderHashes() ([]common.Hash, error) {
	var hashes = make([]common.Hash, 0)
	var seekErr error
	dao.Store.Seek(storage.SeekRange{
		Prefix: dao.mkKeyPrefix(storage.IXHeaderHashList),
	}, func(k, v []byte) bool {
		newHashes, err := read2000Uint256Hashes(v)
		if err != nil {
			seekErr = fmt.Errorf("failed to read batch of 2000 header hashes: %w", err)
			return false
		}
		hashes = append(hashes, newHashes...)
		return true
	})
	return hashes, seekErr
}

// PutVersion stores the given version in the underlying store.
func (dao *Simple) PutVersion(v Version) {
	dao.Version = v
	dao.Store.Put(dao.mkKeyPrefix(storage.SYSVersion), v.Bytes())
}

// PutCurrentHeader stores current header.
func (dao *Simple) PutCurrentHeader(h common.Hash, index uint32) {
	buf := dao.getDataBuf()
	buf.WriteBytes(h.Bytes())
	buf.WriteU32LE(index)
	dao.Store.Put(dao.mkKeyPrefix(storage.SYSCurrentHeader), buf.Bytes())
}

// read2000Uint256Hashes attempts to read 2000 Uint256 hashes from
// the given byte array.
func read2000Uint256Hashes(b []byte) ([]common.Hash, error) {
	r := bytes.NewReader(b)
	br := io.NewBinReaderFromIO(r)
	count := br.ReadVarUint()
	hashes := make([]common.Hash, count)
	for i := uint64(0); i < count; i++ {
		br.ReadBytes(hashes[i][:])
	}
	if br.Err != nil {
		return nil, br.Err
	}
	return hashes, nil
}

func (dao *Simple) mkHeaderHashKey(h uint32) []byte {
	b := dao.getKeyBuf(1 + 4)
	b[0] = byte(storage.IXHeaderHashList)
	binary.BigEndian.PutUint32(b[1:], h)
	return b
}

// StoreHeaderHashes pushes a batch of header hashes into the store.
func (dao *Simple) StoreHeaderHashes(hashes []common.Hash, height uint32) error {
	key := dao.mkHeaderHashKey(height)
	buf := dao.getDataBuf()
	buf.WriteVarUint(uint64(len(hashes)))
	for _, hash := range hashes {
		buf.WriteBytes(hash[:])
	}
	if buf.Err != nil {
		return buf.Err
	}
	dao.Store.Put(key, buf.Bytes())
	return nil
}

// HasTransaction returns nil if the given store does not contain the given
// Transaction hash. It returns an error in case if transaction is in chain
// or in the list of conflicting transactions.
func (dao *Simple) HasTransaction(hash common.Hash) error {
	key := dao.makeTxKey(hash)
	bytes, err := dao.Store.Get(key)
	if err != nil {
		return nil
	}
	if len(bytes) < 4 {
		return nil
	}
	return ErrAlreadyExists
}

// StoreAsCurrentBlock stores a hash of the given block with prefix
// SYSCurrentBlock. It can reuse given buffer for the purpose of value
// serialization.
func (dao *Simple) StoreAsCurrentBlock(block *block.Block) {
	buf := dao.getDataBuf()
	h := block.Hash()
	buf.BinWriter.WriteBytes(h.Bytes())
	buf.WriteU32LE(block.Index)
	dao.Store.Put(dao.mkKeyPrefix(storage.SYSCurrentBlock), buf.Bytes())
}

func (dao *Simple) getKeyBuf(len int) []byte {
	if dao.private {
		if dao.keyBuf == nil {
			dao.keyBuf = make([]byte, 0, 1+4+storage.MaxStorageKeyLen) // Prefix, uint32, key.
		}
		return dao.keyBuf[:len] // Should have enough capacity.
	}
	return make([]byte, len)
}

func (dao *Simple) getDataBuf() *io.BufBinWriter {
	if dao.private {
		if dao.dataBuf == nil {
			dao.dataBuf = io.NewBufBinWriter()
		}
		dao.dataBuf.Reset()
		return dao.dataBuf
	}
	return io.NewBufBinWriter()
}

// Persist flushes all the changes made into the (supposedly) persistent
// underlying store. It doesn't block accesses to DAO from other threads.
func (dao *Simple) Persist() (int, error) {
	return dao.Store.Persist()
}

// PersistSync flushes all the changes made into the (supposedly) persistent
// underlying store. It's a synchronous version of Persist that doesn't allow
// other threads to work with DAO while flushing the Store.
func (dao *Simple) PersistSync() (int, error) {
	return dao.Store.PersistSync()
}
