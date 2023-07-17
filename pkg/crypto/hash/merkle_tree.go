package hash

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// MerkleTree implementation.
type MerkleTree struct {
	root  *MerkleTreeNode
	depth int
}

// NewMerkleTree returns new MerkleTree object.
func NewMerkleTree(hashes []common.Hash) (*MerkleTree, error) {
	if len(hashes) == 0 {
		return nil, errors.New("length of the hashes cannot be zero")
	}

	nodes := make([]*MerkleTreeNode, len(hashes))
	for i := 0; i < len(hashes); i++ {
		nodes[i] = &MerkleTreeNode{
			hash: hashes[i],
		}
	}

	return &MerkleTree{
		root:  buildMerkleTree(nodes),
		depth: 1,
	}, nil
}

// Root returns the computed root hash of the MerkleTree.
func (t *MerkleTree) Root() common.Hash {
	return t.root.hash
}

func (t *MerkleTree) Prove(target common.Hash) ([]common.Hash, uint32, error) {
	var tnode *MerkleTreeNode
	nodes := []*MerkleTreeNode{t.root}
	for len(nodes) > 0 {
		n := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		if !n.IsLeaf() {
			nodes = append(nodes, n.leftChild)
			if n.leftChild != n.rightChild {
				nodes = append(nodes, n.rightChild)
			}
		} else if n.hash == target {
			tnode = n
			break
		}
	}
	if tnode == nil {
		return nil, 0, errors.New("target not found")
	}
	parent := tnode.parent
	hashes := []common.Hash{}
	path := uint32(0)
	for i := 0; parent != nil; i++ {
		if tnode == parent.leftChild {
			path |= (1 << i)
			hashes = append(hashes, parent.rightChild.hash)
		} else {
			hashes = append(hashes, parent.leftChild.hash)
		}
		tnode, parent = parent, parent.parent
	}
	return hashes, path, nil
}

func buildMerkleTree(leaves []*MerkleTreeNode) *MerkleTreeNode {
	if len(leaves) == 0 {
		panic("length of leaves cannot be zero")
	}
	if len(leaves) == 1 {
		return leaves[0]
	}

	parents := make([]*MerkleTreeNode, (len(leaves)+1)/2)
	for i := 0; i < len(parents); i++ {
		parents[i] = &MerkleTreeNode{}
		parents[i].leftChild = leaves[i*2]
		leaves[i*2].parent = parents[i]

		if i*2+1 == len(leaves) {
			parents[i].rightChild = parents[i].leftChild
		} else {
			parents[i].rightChild = leaves[i*2+1]
			leaves[i*2+1].parent = parents[i]
		}

		b1 := parents[i].leftChild.hash.Bytes()
		b2 := parents[i].rightChild.hash.Bytes()
		b1 = append(b1, b2...)
		parents[i].hash = DoubleSha256(b1)
	}

	return buildMerkleTree(parents)
}

// CalcMerkleRoot calculcates Merkle root hash value for a given slice of hashes.
// It doesn't create a full MerkleTree structure and it uses given slice as a
// scratchpad, so it will destroy its contents in the process. But it's much more
// memory efficient if you only need root hash value, while NewMerkleTree would
// make 3*N allocations for N hashes, this function will only make 4. It also is
// an error to call this function for zero-length hashes slice, the function will
// panic.
func CalcMerkleRoot(hashes []common.Hash) common.Hash {
	if len(hashes) == 0 {
		return common.Hash{}
	}
	if len(hashes) == 1 {
		return hashes[0]
	}

	scratch := make([]byte, 64)
	parents := hashes[:(len(hashes)+1)/2]
	for i := 0; i < len(parents); i++ {
		copy(scratch, hashes[i*2].Bytes())

		if i*2+1 == len(hashes) {
			copy(scratch[32:], hashes[i*2].Bytes())
		} else {
			copy(scratch[32:], hashes[i*2+1].Bytes())
		}

		parents[i] = DoubleSha256(scratch)
	}

	return CalcMerkleRoot(parents)
}

// MerkleTreeNode represents a node in the MerkleTree.
type MerkleTreeNode struct {
	hash       common.Hash
	parent     *MerkleTreeNode
	leftChild  *MerkleTreeNode
	rightChild *MerkleTreeNode
}

// IsLeaf returns whether this node is a leaf node or not.
func (n *MerkleTreeNode) IsLeaf() bool {
	return n.leftChild == nil && n.rightChild == nil
}

// IsRoot returns whether this node is a root node or not.
func (n *MerkleTreeNode) IsRoot() bool {
	return n.parent == nil
}

func VerifyMerkleProof(root common.Hash, target common.Hash, hashes []common.Hash, path uint32) bool {
	return root == CalcProofRoot(target, hashes, path)
}

func CalcProofRoot(target common.Hash, hashes []common.Hash, path uint32) common.Hash {
	scratch := make([]byte, 64)
	parent := target
	for i := 0; i < len(hashes); i++ {
		fmt.Printf("hash: %s\n", base64.StdEncoding.EncodeToString(hashes[i][:]))
		if (path >> i & 1) == 1 {
			copy(scratch, parent[:])
			copy(scratch[32:], hashes[i][:])
		} else {
			copy(scratch, hashes[i][:])
			copy(scratch[32:], parent[:])
		}
		parent = DoubleSha256(scratch)
		fmt.Printf("parent %s\n", base64.StdEncoding.EncodeToString(scratch))
	}
	return parent
}
