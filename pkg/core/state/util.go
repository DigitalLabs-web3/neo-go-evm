package state

import (
	"encoding/binary"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func CreateContractHash(sender common.Address, nonce uint64, codeHash common.Hash) common.Hash {
	data := make([]byte, common.AddressLength+8+common.HashLength)
	copy(data, sender.Bytes())
	binary.LittleEndian.PutUint64(data[common.AddressLength:], nonce)
	copy(data[(common.AddressLength+8):], codeHash.Bytes())
	return hash.Keccak256(data)
}

func EncodeLog(l *types.Log, w *io.BinWriter) {
	w.WriteBytes(l.Address[:])
	w.WriteVarUint(uint64(len(l.Topics)))
	for _, t := range l.Topics {
		w.WriteBytes(t[:])
	}
	w.WriteVarBytes(l.Data)
	w.WriteU64LE(l.BlockNumber)
	w.WriteBytes(l.TxHash[:])
	w.WriteU32LE(uint32(l.TxIndex))
	w.WriteBytes(l.BlockHash[:])
	w.WriteU32LE(uint32(l.Index))
}

func DecodeLog(l *types.Log, r *io.BinReader) {
	r.ReadBytes(l.Address[:])
	count := r.ReadVarUint()
	l.Topics = make([]common.Hash, count)
	for i := uint64(0); i < count; i++ {
		r.ReadBytes(l.Topics[i][:])
	}
	l.Data = r.ReadVarBytes()
	l.BlockNumber = r.ReadU64LE()
	r.ReadBytes(l.TxHash[:])
	l.TxIndex = uint(r.ReadU32LE())
	r.ReadBytes(l.BlockHash[:])
	l.Index = uint(r.ReadU64LE())
	l.Removed = false
}
