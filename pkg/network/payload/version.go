package payload

import (
	"time"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/network/capability"
)

// MaxUserAgentLength is the limit for user agent field.
const MaxUserAgentLength = 1024

// Version payload.
type Version struct {
	// NetMode of the node
	ChainID uint64
	// currently the version of the protocol is 0
	Version uint32
	// timestamp
	Timestamp uint32
	// it's used to distinguish the node from public IP
	Nonce uint32
	// client id
	UserAgent []byte
	// List of available network services
	Capabilities capability.Capabilities
}

// NewVersion returns a pointer to a Version payload.
func NewVersion(chainId uint64, id uint32, ua string, c []capability.Capability) *Version {
	return &Version{
		ChainID:      chainId,
		Version:      0,
		Timestamp:    uint32(time.Now().UTC().Unix()),
		Nonce:        id,
		UserAgent:    []byte(ua),
		Capabilities: c,
	}
}

// DecodeBinary implements Serializable interface.
func (p *Version) DecodeBinary(br *io.BinReader) {
	p.ChainID = br.ReadU64LE()
	p.Version = br.ReadU32LE()
	p.Timestamp = br.ReadU32LE()
	p.Nonce = br.ReadU32LE()
	p.UserAgent = br.ReadVarBytes(MaxUserAgentLength)
	p.Capabilities.DecodeBinary(br)
}

// EncodeBinary implements Serializable interface.
func (p *Version) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU64LE(p.ChainID)
	bw.WriteU32LE(p.Version)
	bw.WriteU32LE(p.Timestamp)
	bw.WriteU32LE(p.Nonce)
	bw.WriteVarBytes(p.UserAgent)
	p.Capabilities.EncodeBinary(bw)
}
