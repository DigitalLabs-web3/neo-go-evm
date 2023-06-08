package payload

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
)

// The node can broadcast the object information it owns by this message.
// The message can be sent automatically or can be used to answer getblock messages.

// InventoryType is the type of an object in the Inventory message.
type InventoryType uint8

// String implements the Stringer interface.
func (i InventoryType) String() string {
	switch i {
	case TXType:
		return "TX"
	case BlockType:
		return "block"
	case ExtensibleType:
		return "extensible"
	case P2PNotaryRequestType:
		return "p2pNotaryRequest"
	default:
		return "unknown inventory type"
	}
}

// Valid returns true if the inventory (type) is known.
func (i InventoryType) Valid() bool {
	return i == BlockType || i == TXType || i == ExtensibleType
}

// List of valid InventoryTypes.
const (
	TXType               InventoryType = 0x2b
	BlockType            InventoryType = 0x2c
	ExtensibleType       InventoryType = 0x2e
	P2PNotaryRequestType InventoryType = 0x50
)

// Inventory payload.
type Inventory struct {
	// Type if the object hash.
	Type InventoryType

	// A list of hashes.
	Hashes []common.Hash
}

// NewInventory return a pointer to an Inventory.
func NewInventory(typ InventoryType, hashes []common.Hash) *Inventory {
	return &Inventory{
		Type:   typ,
		Hashes: hashes,
	}
}

// DecodeBinary implements Serializable interface.
func (p *Inventory) DecodeBinary(br *io.BinReader) {
	p.Type = InventoryType(br.ReadB())
	count := br.ReadVarUint()
	p.Hashes = make([]common.Hash, count)
	for i := uint64(0); i < count; i++ {
		br.ReadBytes(p.Hashes[i][:])
	}
}

// EncodeBinary implements Serializable interface.
func (p *Inventory) EncodeBinary(bw *io.BinWriter) {
	bw.WriteB(byte(p.Type))
	bw.WriteVarUint(uint64(len(p.Hashes)))
	for _, hash := range p.Hashes {
		bw.WriteBytes(hash[:])
	}
}
