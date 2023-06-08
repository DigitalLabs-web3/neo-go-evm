package stateroot

import (
	"sync"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/consensus"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/network/payload"
)

type (
	incompleteRoot struct {
		sync.RWMutex
		// svList is a list of state validator keys for this stateroot.
		svList keys.PublicKeys
		// isSent is true state root was already broadcasted.
		isSent bool
		// request is oracle request.
		root *state.MPTRoot
		// sigs contains signature from every oracle node.
		sigs map[string]*rootSig
		// myIndex is the index of validator for this root.
		myIndex int
		// myVote is an extensible message containing node's vote.
		myVote *payload.Extensible
		// retries is a counter of send attempts.
		retries int
	}

	rootSig struct {
		// pub is cached public key.
		pub *keys.PublicKey
		// ok is true if signature was verified.
		ok bool
		// sig is state root signature.
		sig []byte
	}
)

func (r *incompleteRoot) reverify(chainId uint64) {
	for _, sig := range r.sigs {
		if !sig.ok {
			sig.ok = sig.pub.VerifyHashable(sig.sig, chainId, r.root)
		}
	}
}

func (r *incompleteRoot) addSignature(pub *keys.PublicKey, sig []byte) {
	r.sigs[string(pub.Bytes())] = &rootSig{
		pub: pub,
		ok:  r.root != nil,
		sig: sig,
	}
}

func (r *incompleteRoot) isSenderNow() bool {
	if r.root == nil || r.isSent || len(r.svList) == 0 {
		return false
	}
	retries := r.retries
	if retries < 0 {
		retries = 0
	}
	ind := (int(r.root.Index) - retries) % len(r.svList)
	if ind < 0 {
		ind += len(r.svList)
	}
	return ind == r.myIndex
}

// finalize checks is either main or backup tx has sufficient number of signatures and returns
// tx and bool value indicating if it is ready to be broadcasted.
func (r *incompleteRoot) finalize() (*state.MPTRoot, bool) {
	if r.root == nil {
		return nil, false
	}

	m := consensus.GetDefaultHonestNodeCount(len(r.svList))
	sigs := make([][]byte, 0, m)
	for _, pub := range r.svList {
		sig, ok := r.sigs[string(pub.Bytes())]
		if ok && sig.ok {
			sigs = append(sigs, sig.sig)
			if len(sigs) == m {
				break
			}
		}
	}
	if len(sigs) != m {
		return nil, false
	}
	verification, err := r.svList.CreateDefaultMultiSigRedeemScript()
	if err != nil {
		return nil, false
	}
	invocation := crypto.CreateMultiInvocationScript(sigs)
	r.root.Witness = transaction.Witness{
		VerificationScript: verification,
		InvocationScript:   invocation,
	}
	return r.root, true
}
