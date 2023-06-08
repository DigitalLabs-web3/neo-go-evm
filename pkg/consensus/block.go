package consensus

import (
	"errors"

	coreb "github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/dbft/block"
	"github.com/ethereum/go-ethereum/common"
)

// methods necessary for dBFT library.
type consensusBlock struct {
	coreb.Block

	chainId   uint64
	signature []byte
}

var _ block.Block = (*consensusBlock)(nil)

// Sign implements block.Block interface.
func (n *consensusBlock) Sign(key *keys.PrivateKey) error {
	sig := key.SignHashable(n.chainId, &n.Block)
	n.signature = sig
	return nil
}

// Verify implements block.Block interface.
func (n *consensusBlock) Verify(key *keys.PublicKey, sign []byte) error {
	if key.VerifyHashable(sign, n.chainId, &n.Block) {
		return nil
	}
	return errors.New("verification failed")
}

// Transactions implements block.Block interface.
func (n *consensusBlock) Transactions() []block.Transaction {
	txes := make([]block.Transaction, len(n.Block.Transactions))
	for i, tx := range n.Block.Transactions {
		txes[i] = tx
	}

	return txes
}

// SetTransactions implements block.Block interface.
func (n *consensusBlock) SetTransactions(txes []block.Transaction) {
	n.Block.Transactions = make([]*transaction.Transaction, len(txes))
	for i, tx := range txes {
		n.Block.Transactions[i] = tx.(*transaction.Transaction)
	}
}

// Version implements block.Block interface.
func (n *consensusBlock) Version() uint32 { return n.Block.Version }

// PrevHash implements block.Block interface.
func (n *consensusBlock) PrevHash() common.Hash { return n.Block.PrevHash }

// MerkleRoot implements block.Block interface.
func (n *consensusBlock) MerkleRoot() common.Hash { return n.Block.MerkleRoot }

// Timestamp implements block.Block interface.
func (n *consensusBlock) Timestamp() uint64 { return n.Block.Timestamp * nsInMs }

// Index implements block.Block interface.
func (n *consensusBlock) Index() uint32 { return n.Block.Index }

// ConsensusData implements block.Block interface.
func (n *consensusBlock) ConsensusData() uint64 { return n.Block.Nonce }

// NextConsensus implements block.Block interface.
func (n *consensusBlock) NextConsensus() common.Address { return n.Block.NextConsensus }

// Signature implements block.Block interface.
func (n *consensusBlock) Signature() []byte { return n.signature }
