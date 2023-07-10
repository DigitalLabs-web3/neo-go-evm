package mempool

import (
	"errors"
	"math/big"
	"math/bits"
	"sort"
	"sync"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/mempoolevent"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/atomic"
)

var (
	// ErrInsufficientFunds is returned when Sender is not able to pay for
	// transaction being added irrespective of the other contents of the
	// pool.
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrConflict is returned when transaction being added is incompatible
	// with the contents of the memory pool (Sender doesn't have enough GAS
	// to pay for all transactions in the pool).
	ErrConflict = errors.New("conflicts: insufficient funds for all pooled tx")
	// ErrDup is returned when transaction being added is already present
	// in the memory pool.
	ErrDup = errors.New("already in the memory pool")
	// ErrOOM is returned when transaction just doesn't fit in the memory
	// pool because of its capacity constraints.
	ErrOOM = errors.New("out of memory")
	// ErrConflictsAttribute is returned when transaction conflicts with other transactions
	// due to its (or theirs) nonce.
	ErrConflictsNonce = errors.New("conflicts with memory pool due to nonce")
)

// item represents a transaction in the the Memory pool.
type item struct {
	txn        *transaction.Transaction
	blockStamp uint32
	data       interface{}
	priority   big.Int
}

// items is a slice of item.
type items []item

// utilityBalanceAndFees stores sender's balance and overall fees of
// sender's transactions which are currently in mempool.
type utilityBalanceAndFees struct {
	balance big.Int
	feeSum  big.Int
}

// Pool stores the unconfirms transactions.
type Pool struct {
	lock         sync.RWMutex
	verifiedMap  map[common.Hash]*transaction.Transaction
	verifiedTxes items
	fees         map[common.Address]utilityBalanceAndFees

	pendingNonces map[common.Address]uint64
	senderMap     map[common.Address]map[uint64]*item

	capacity   int
	feePerByte uint64
	payerIndex int

	resendThreshold uint32
	resendFunc      func(*transaction.Transaction, interface{})

	// subscriptions for mempool events
	subscriptionsEnabled bool
	subscriptionsOn      atomic.Bool
	stopCh               chan struct{}
	events               chan mempoolevent.Event
	subCh                chan chan<- mempoolevent.Event // there are no other events in mempool except Event, so no need in generic subscribers type
	unsubCh              chan chan<- mempoolevent.Event
}

func (p items) Len() int           { return len(p) }
func (p items) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p items) Less(i, j int) bool { return p[i].CompareTo(p[j]) < 0 }

// CompareTo returns the difference between two items.
// difference < 0 implies p < otherP.
// difference = 0 implies p = otherP.
// difference > 0 implies p > otherP.
func (p item) CompareTo(otherP item) int {
	// Fees sorted ascending.
	if ret := p.txn.GasPrice().Cmp(otherP.txn.GasPrice()); ret != 0 {
		return ret
	}
	if p.txn.Gas() > otherP.txn.Gas() {
		return 1
	} else if p.txn.Gas() < otherP.txn.Gas() {
		return -1
	}
	return p.txn.Hash().Big().Cmp(otherP.txn.Hash().Big())
}

// Count returns the total number of uncofirm transactions.
func (mp *Pool) Count() int {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	return mp.count()
}

// count is an internal unlocked version of Count.
func (mp *Pool) count() int {
	return len(mp.verifiedTxes)
}

// ContainsKey checks if a transactions hash is in the Pool.
func (mp *Pool) ContainsKey(hash common.Hash) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	return mp.containsKey(hash)
}

// containsKey is an internal unlocked version of ContainsKey.
func (mp *Pool) containsKey(hash common.Hash) bool {
	if _, ok := mp.verifiedMap[hash]; ok {
		return true
	}

	return false
}

func (mp *Pool) HasConflicts(t *transaction.Transaction, fee Feer) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	return mp.containsKey(t.Hash())
}

// tryAddSendersFee tries to add system fee and network fee to the total sender`s fee in mempool
// and returns false if both balance check is required and sender has not enough GAS to pay.
func (mp *Pool) tryAddSendersFee(tx *transaction.Transaction, feer Feer, needCheck bool) bool {
	payer := tx.From()
	senderFee, ok := mp.fees[payer]
	if !ok {
		senderFee.balance = *(feer.GetUtilityTokenBalance(payer))
		mp.fees[payer] = senderFee
	}
	if needCheck {
		newFeeSum, err := checkBalance(tx, senderFee)
		if err != nil {
			return false
		}
		senderFee.feeSum = newFeeSum
	} else {
		(&senderFee.feeSum).Add(&senderFee.feeSum, tx.Cost())
	}
	mp.fees[payer] = senderFee
	return true
}

// checkBalance returns new cumulative fee balance for account or an error in
// case sender doesn't have enough GAS to pay for the transaction.
func checkBalance(tx *transaction.Transaction, balance utilityBalanceAndFees) (big.Int, error) {
	var txFee big.Int = *tx.Cost()
	if balance.balance.Cmp(&txFee) < 0 {
		return txFee, ErrInsufficientFunds
	}
	(&txFee).Add(&txFee, &balance.feeSum)
	if balance.balance.Cmp(&txFee) < 0 {
		return txFee, ErrConflict
	}
	return txFee, nil
}

// Add tries to add given transaction to the Pool.
func (mp *Pool) Add(t *transaction.Transaction, fee Feer, data ...interface{}) error {
	var pItem = item{
		txn:        t,
		blockStamp: fee.BlockHeight(),
		priority:   *t.GasPrice(),
	}
	if data != nil {
		pItem.data = data[0]
	}
	mp.lock.Lock()
	defer mp.lock.Unlock()
	if mp.containsKey(t.Hash()) {
		return ErrDup
	}
	conflict, err := mp.checkTxConflicts(t, fee)
	if err != nil {
		return err
	}
	if conflict != nil {
		mp.removeInternal(conflict, fee)
	}
	// if pre pending tx of same sender exists, set priority to previous tx
	pre, ok := mp.senderMap[pItem.txn.From()][pItem.txn.Nonce()-1]
	if ok && pItem.priority.Cmp(&pre.priority) == 1 {
		pItem.priority = pre.priority
	}
	// Insert into sorted array (from max to min, that could also be done
	// using sort.Sort(sort.Reverse()), but it incurs more overhead. Notice
	// also that we're searching for position that is strictly more
	// prioritized than our new item because we do expect a lot of
	// transactions with the same priority and appending to the end of the
	// slice is always more efficient.
	n := sort.Search(len(mp.verifiedTxes), func(n int) bool {
		return pItem.priority.Cmp(&mp.verifiedTxes[n].priority) > 0
	})

	// We've reached our capacity already.
	if len(mp.verifiedTxes) == mp.capacity {
		// Less prioritized than the least prioritized we already have, won't fit.
		if n == len(mp.verifiedTxes) {
			return ErrOOM
		}
		// Ditch the last one.
		unlucky := mp.verifiedTxes[len(mp.verifiedTxes)-1]
		delete(mp.verifiedMap, unlucky.txn.Hash())
		delete(mp.senderMap[unlucky.txn.From()], unlucky.txn.Nonce())
		mp.pendingNonces[unlucky.txn.From()]--

		mp.verifiedTxes[len(mp.verifiedTxes)-1] = pItem
		if mp.subscriptionsOn.Load() {
			mp.events <- mempoolevent.Event{
				Type: mempoolevent.TransactionRemoved,
				Tx:   unlucky.txn,
				Data: unlucky.data,
			}
		}
	} else {
		mp.verifiedTxes = append(mp.verifiedTxes, pItem)
	}
	if n != len(mp.verifiedTxes)-1 {
		copy(mp.verifiedTxes[n+1:], mp.verifiedTxes[n:])
		mp.verifiedTxes[n] = pItem
	}
	mp.verifiedMap[t.Hash()] = t
	if mp.senderMap[t.From()] == nil {
		mp.senderMap[t.From()] = make(map[uint64]*item)
	}
	mp.senderMap[t.From()][t.Nonce()] = &pItem
	// only new tx need change nonce
	if conflict == nil {
		mp.pendingNonces[t.From()] = t.Nonce() + 1
	}
	// we already checked balance in checkTxConflicts, so don't need to check again
	mp.tryAddSendersFee(pItem.txn, fee, false)

	updateMempoolMetrics(len(mp.verifiedTxes))
	mp.lock.Unlock()

	if mp.subscriptionsOn.Load() {
		mp.events <- mempoolevent.Event{
			Type: mempoolevent.TransactionAdded,
			Tx:   pItem.txn,
			Data: pItem.data,
		}
	}
	return nil
}

// get pending nonce of from
func (mp *Pool) PendingNonce(addr common.Address) uint64 {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	return mp.pendingNonces[addr]
}

// removeInternal is an internal unlocked representation of Remove.
// don't change nonce
func (mp *Pool) removeInternal(tx *transaction.Transaction, feer Feer) {
	var num int
	delete(mp.verifiedMap, tx.Hash())
	delete(mp.senderMap[tx.From()], tx.Nonce())

	for num = range mp.verifiedTxes {
		if tx.Hash() == (mp.verifiedTxes[num].txn.Hash()) {
			break
		}
	}
	itm := mp.verifiedTxes[num]
	if num < len(mp.verifiedTxes)-1 {
		mp.verifiedTxes = append(mp.verifiedTxes[:num], mp.verifiedTxes[num+1:]...)
	} else if num == len(mp.verifiedTxes)-1 {
		mp.verifiedTxes = mp.verifiedTxes[:num]
	}
	payer := itm.txn.From()
	senderFee := mp.fees[payer]
	(&senderFee.feeSum).Sub(&senderFee.feeSum, tx.Cost())
	mp.fees[payer] = senderFee
	if mp.subscriptionsOn.Load() {
		mp.events <- mempoolevent.Event{
			Type: mempoolevent.TransactionRemoved,
			Tx:   itm.txn,
			Data: itm.data,
		}
	}
	updateMempoolMetrics(len(mp.verifiedTxes))
}

// RemoveStale filters verified transactions through the given function keeping
// only the transactions for which it returns a true result. It's used to quickly
// drop part of the mempool that is now invalid after the block acceptance.
func (mp *Pool) RemoveStale(isOK func(*transaction.Transaction) bool, feer Feer) {
	mp.lock.Lock()
	policyChanged := mp.loadPolicy(feer)
	// We can reuse already allocated slice
	// because items are iterated one-by-one in increasing order.
	newVerifiedTxes := mp.verifiedTxes[:0]
	mp.fees = make(map[common.Address]utilityBalanceAndFees) // it'd be nice to reuse existing map, but we can't easily clear it
	height := feer.BlockHeight()
	var (
		staleItems []item
	)
	for _, itm := range mp.verifiedTxes {
		if isOK(itm.txn) && mp.checkPolicy(itm.txn, policyChanged) && mp.tryAddSendersFee(itm.txn, feer, true) {
			newVerifiedTxes = append(newVerifiedTxes, itm)
			if mp.resendThreshold != 0 {
				// item is resend at resendThreshold, 2*resendThreshold, 4*resendThreshold ...
				// so quotient must be a power of two.
				diff := (height - itm.blockStamp)
				if diff%mp.resendThreshold == 0 && bits.OnesCount32(diff/mp.resendThreshold) == 1 {
					staleItems = append(staleItems, itm)
				}
			}
		} else {
			delete(mp.verifiedMap, itm.txn.Hash())
			delete(mp.senderMap[itm.txn.From()], itm.txn.Nonce())
			if mp.subscriptionsOn.Load() {
				mp.events <- mempoolevent.Event{
					Type: mempoolevent.TransactionRemoved,
					Tx:   itm.txn,
					Data: itm.data,
				}
			}
		}
	}
	if len(staleItems) != 0 {
		go mp.resendStaleItems(staleItems)
	}
	mp.verifiedTxes = newVerifiedTxes
	mp.lock.Unlock()
}

// loadPolicy updates feePerByte field and returns whether policy has been
// changed.
func (mp *Pool) loadPolicy(feer Feer) bool {
	newFeePerByte := feer.FeePerByte()
	if newFeePerByte > mp.feePerByte {
		mp.feePerByte = newFeePerByte
		return true
	}
	return false
}

// checkPolicy checks whether transaction fits policy.
func (mp *Pool) checkPolicy(tx *transaction.Transaction, policyChanged bool) bool {
	if !policyChanged || tx.FeePerByte() >= mp.feePerByte {
		return true
	}
	return false
}

// New returns a new Pool struct.
func New(capacity int, payerIndex int, enableSubscriptions bool) *Pool {
	mp := &Pool{
		verifiedMap:          make(map[common.Hash]*transaction.Transaction, capacity),
		senderMap:            make(map[common.Address]map[uint64]*item, capacity/2),
		verifiedTxes:         make([]item, 0, capacity),
		capacity:             capacity,
		payerIndex:           payerIndex,
		fees:                 make(map[common.Address]utilityBalanceAndFees),
		subscriptionsEnabled: enableSubscriptions,
		stopCh:               make(chan struct{}),
		events:               make(chan mempoolevent.Event),
		subCh:                make(chan chan<- mempoolevent.Event),
		unsubCh:              make(chan chan<- mempoolevent.Event),
	}
	mp.subscriptionsOn.Store(false)
	return mp
}

// SetResendThreshold sets threshold after which transaction will be considered stale
// and returned for retransmission by `GetStaleTransactions`.
func (mp *Pool) SetResendThreshold(h uint32, f func(*transaction.Transaction, interface{})) {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	mp.resendThreshold = h
	mp.resendFunc = f
}

func (mp *Pool) resendStaleItems(items []item) {
	for i := range items {
		mp.resendFunc(items[i].txn, items[i].data)
	}
}

// TryGetValue returns a transaction and its fee if it exists in the memory pool.
func (mp *Pool) TryGetValue(hash common.Hash) (*transaction.Transaction, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	if tx, ok := mp.verifiedMap[hash]; ok {
		return tx, ok
	}

	return nil, false
}

// TryGetData returns data associated with the specified transaction if it exists in the memory pool.
func (mp *Pool) TryGetData(hash common.Hash) (interface{}, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	if tx, ok := mp.verifiedMap[hash]; ok {
		itm := item{txn: tx}
		n := sort.Search(len(mp.verifiedTxes), func(n int) bool {
			return itm.CompareTo(mp.verifiedTxes[n]) >= 0
		})
		if n < len(mp.verifiedTxes) {
			for i := n; i < len(mp.verifiedTxes); i++ { // items may have equal priority, so `n` is the left bound of the items which are as prioritized as the desired `itm`.
				if mp.verifiedTxes[i].txn.Hash() == hash {
					return mp.verifiedTxes[i].data, ok
				}
				if itm.CompareTo(mp.verifiedTxes[i]) != 0 {
					break
				}
			}
		}
	}

	return nil, false
}

// GetVerifiedTransactions returns a slice of transactions with their fees.
func (mp *Pool) GetVerifiedTransactions() []*transaction.Transaction {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	var t = make([]*transaction.Transaction, len(mp.verifiedTxes))

	for i := range mp.verifiedTxes {
		t[i] = mp.verifiedTxes[i].txn
	}

	return t
}

// checkTxConflicts is an internal unprotected version of Verify. It takes into
// consideration conflicting transactions which are about to be removed from mempool.
func (mp *Pool) checkTxConflicts(tx *transaction.Transaction, fee Feer) (*transaction.Transaction, error) {
	payer := tx.From()
	actualSenderFee, ok := mp.fees[payer]
	if !ok {
		actualSenderFee.balance = *(fee.GetUtilityTokenBalance(payer))
	}
	var expectedSenderFee = actualSenderFee
	// Check Conflicts attributes.
	var conflictToBeRemoved *transaction.Transaction
	if existTx, ok := mp.senderMap[tx.From()][tx.Nonce()]; ok {
		if existTx.txn.GasPrice().Cmp(tx.GasPrice()) < 0 {
			conflictToBeRemoved = existTx.txn
			(&expectedSenderFee.feeSum).Sub(&expectedSenderFee.feeSum, existTx.txn.Cost())
		} else {
			return nil, ErrConflictsNonce
		}
	}
	_, err := checkBalance(tx, expectedSenderFee)
	return conflictToBeRemoved, err
}

// Verify checks if a Sender of tx is able to pay for it (and all the other
// transactions in the pool). If yes, the transaction tx is a valid
// transaction and the function returns true. If no, the transaction tx is
// considered to be invalid the function returns false.
func (mp *Pool) Verify(tx *transaction.Transaction, feer Feer) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	_, err := mp.checkTxConflicts(tx, feer)
	return err == nil
}
