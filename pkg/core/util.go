package core

import (
	"math/big"
	"time"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/ethereum/go-ethereum/common"
)

// createGenesisBlock creates a genesis block based on the given configuration.
func createGenesisBlock(cfg *config.ProtocolConfiguration) (*block.Block, error) {
	nextConsensus, err := getConsensusAddress(cfg.StandbyValidators)
	if err != nil {
		return nil, err
	}
	base := block.Header{
		Version:       0,
		PrevHash:      common.Hash{},
		Timestamp:     uint64(time.Date(2016, 7, 15, 15, 8, 21, 0, time.UTC).Unix()), // Milliseconds.
		Nonce:         2083236893,
		Index:         0,
		NextConsensus: nextConsensus,
		Witness: transaction.Witness{
			VerificationScript: []byte{},
			InvocationScript:   []byte{},
		},
	}
	h := hash.Keccak256([]byte("initialize()"))
	initData := h[:4]
	gas := (transaction.EthLegacyBaseLength + 4) * native.DefaultFeePerByte
	gasPrice := big.NewInt(int64(native.DefaultGasPrice))
	from := common.HexToAddress("01")
	b := &block.Block{
		Header: base,
		Transactions: []*transaction.Transaction{
			transaction.NewTx(&transaction.NeoTx{
				Nonce:    0,
				GasPrice: gasPrice,
				Gas:      gas,
				From:     from,
				To:       &native.DesignationAddress,
				Data:     initData,
				Value:    big.NewInt(0),
				Witness: transaction.Witness{
					InvocationScript:   []byte{0},
					VerificationScript: []byte{0},
				},
			}),
			transaction.NewTx(&transaction.NeoTx{
				Nonce:    0,
				GasPrice: gasPrice,
				Gas:      gas,
				From:     from,
				To:       &native.PolicyAddress,
				Data:     initData,
				Value:    big.NewInt(0),
				Witness: transaction.Witness{
					InvocationScript:   []byte{0},
					VerificationScript: []byte{0},
				},
			}),
			transaction.NewTx(&transaction.NeoTx{
				Nonce:    0,
				GasPrice: gasPrice,
				Gas:      gas,
				From:     from,
				To:       &native.GASAddress,
				Data:     initData,
				Value:    big.NewInt(0),
				Witness: transaction.Witness{
					InvocationScript:   []byte{0},
					VerificationScript: []byte{0},
				},
			}),
			transaction.NewTx(&transaction.NeoTx{
				GasPrice: gasPrice,
				Gas:      gas,
				From:     from,
				To:       &native.ManagementAddress,
				Data:     initData,
				Value:    big.NewInt(0),
				Witness: transaction.Witness{
					InvocationScript:   []byte{0},
					VerificationScript: []byte{0},
				},
			}),
		},
	}
	b.RebuildMerkleRoot()

	return b, nil
}

func getConsensusAddress(validators []*keys.PublicKey) (val common.Address, err error) {
	raw, err := keys.PublicKeys(validators).CreateDefaultMultiSigRedeemScript()
	if err != nil {
		return val, err
	}
	return hash.Hash160(raw), nil
}

// headerSliceReverse reverses the given slice of *Header.
func headerSliceReverse(dest []*block.Header) {
	for i, j := 0, len(dest)-1; i < j; i, j = i+1, j-1 {
		dest[i], dest[j] = dest[j], dest[i]
	}
}
