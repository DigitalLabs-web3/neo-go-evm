package core

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/smartcontract/trigger"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

/*  Missing tests:
 *  TestTxGetReferences
 *  TestTxGetUnspentCoins
 *  TestTxGetWitnesses
 *  TestBcGetAccount
 *  TestBcGetAsset
 *  TestAccountGetBalance
 *  TestAccountIsStandard
 *  TestCreateContractStateFromVM
 *  TestContractCreate
 *  TestContractMigrate
 *  TestAssetCreate
 *  TestAssetRenew
 *  TestRuntimeSerialize
 *  TestRuntimeDeserialize
 */

func TestHeaderGetVersion(t *testing.T) {
	v, block, context := createVMAndPushBlock(t)

	err := context.headerGetVersion(v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().(*big.Int)
	require.Equal(t, uint64(block.Version), value.Uint64())
}

func TestHeaderGetVersion_Negative(t *testing.T) {
	v := vm.New()
	block := newDumbBlock()
	context := newInteropContext(trigger.Application, newTestChain(t), storage.NewMemoryStore(), block, nil)
	v.Estack().PushVal(vm.NewBoolItem(false))

	err := context.headerGetVersion(v)
	require.Errorf(t, err, "value is not a header or block")
}

func TestHeaderGetConsensusData(t *testing.T) {
	v, block, context := createVMAndPushBlock(t)

	err := context.headerGetConsensusData(v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().(*big.Int)
	require.Equal(t, block.ConsensusData, value.Uint64())
}

func TestHeaderGetMerkleRoot(t *testing.T) {
	v, block, context := createVMAndPushBlock(t)

	err := context.headerGetMerkleRoot(v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value()
	require.Equal(t, block.MerkleRoot.BytesReverse(), value)
}

func TestHeaderGetNextConsensus(t *testing.T) {
	v, block, context := createVMAndPushBlock(t)

	err := context.headerGetNextConsensus(v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value()
	require.Equal(t, block.NextConsensus.BytesReverse(), value)
}

func TestTxGetAttributes(t *testing.T) {
	v, tx, context := createVMAndPushTX(t)

	err := context.txGetAttributes(v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]vm.StackItem)
	require.Equal(t, tx.Attributes[0].Usage, value[0].Value().(*transaction.Attribute).Usage)
}

func TestTxGetInputs(t *testing.T) {
	v, tx, context := createVMAndPushTX(t)

	err := context.txGetInputs(v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]vm.StackItem)
	require.Equal(t, tx.Inputs[0], value[0].Value().(*transaction.Input))
}

func TestTxGetOutputs(t *testing.T) {
	v, tx, context := createVMAndPushTX(t)

	err := context.txGetOutputs(v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]vm.StackItem)
	require.Equal(t, tx.Outputs[0], value[0].Value().(*transaction.Output))
}

func TestTxGetType(t *testing.T) {
	v, tx, context := createVMAndPushTX(t)

	err := context.txGetType(v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().(*big.Int)
	require.Equal(t, big.NewInt(int64(tx.Type)), value)
}

func TestPopInputFromVM(t *testing.T) {
	v, tx, _ := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Inputs[0]))

	input, err := popInputFromVM(v)
	require.NoError(t, err)
	require.Equal(t, tx.Inputs[0], input)
}

func TestInputGetHash(t *testing.T) {
	v, tx, context := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Inputs[0]))

	err := context.inputGetHash(v)
	require.NoError(t, err)
	hash := v.Estack().Pop().Value()
	require.Equal(t, tx.Inputs[0].PrevHash.Bytes(), hash)
}

func TestInputGetIndex(t *testing.T) {
	v, tx, context := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Inputs[0]))

	err := context.inputGetIndex(v)
	require.NoError(t, err)
	index := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(tx.Inputs[0].PrevIndex)), index)
}

func TestPopOutputFromVM(t *testing.T) {
	v, tx, _ := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Outputs[0]))

	output, err := popOutputFromVM(v)
	require.NoError(t, err)
	require.Equal(t, tx.Outputs[0], output)
}

func TestOutputGetAssetID(t *testing.T) {
	v, tx, context := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Outputs[0]))

	err := context.outputGetAssetID(v)
	require.NoError(t, err)
	assetID := v.Estack().Pop().Value()
	require.Equal(t, tx.Outputs[0].AssetID.Bytes(), assetID)
}

func TestOutputGetScriptHash(t *testing.T) {
	v, tx, context := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Outputs[0]))

	err := context.outputGetScriptHash(v)
	require.NoError(t, err)
	scriptHash := v.Estack().Pop().Value()
	require.Equal(t, tx.Outputs[0].ScriptHash.Bytes(), scriptHash)
}

func TestOutputGetValue(t *testing.T) {
	v, tx, context := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Outputs[0]))

	err := context.outputGetValue(v)
	require.NoError(t, err)
	amount := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(tx.Outputs[0].Amount)), amount)
}

func TestAttrGetData(t *testing.T) {
	v, tx, context := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Attributes[0]))

	err := context.attrGetData(v)
	require.NoError(t, err)
	data := v.Estack().Pop().Value()
	require.Equal(t, tx.Attributes[0].Data, data)
}

func TestAttrGetUsage(t *testing.T) {
	v, tx, context := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx.Attributes[0]))

	err := context.attrGetUsage(v)
	require.NoError(t, err)
	usage := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(tx.Attributes[0].Usage)), usage)
}

func TestAccountGetScriptHash(t *testing.T) {
	v, accState, context := createVMAndAccState(t)
	v.Estack().PushVal(vm.NewInteropItem(accState))

	err := context.accountGetScriptHash(v)
	require.NoError(t, err)
	hash := v.Estack().Pop().Value()
	require.Equal(t, accState.ScriptHash.Bytes(), hash)
}

func TestAccountGetVotes(t *testing.T) {
	v, accState, context := createVMAndAccState(t)
	v.Estack().PushVal(vm.NewInteropItem(accState))

	err := context.accountGetVotes(v)
	require.NoError(t, err)
	votes := v.Estack().Pop().Value().([]vm.StackItem)
	require.Equal(t, vm.NewByteArrayItem(accState.Votes[0].Bytes()), votes[0])
}

func TestContractGetScript(t *testing.T) {
	v, contractState, context := createVMAndContractState(t)
	v.Estack().PushVal(vm.NewInteropItem(contractState))

	err := context.contractGetScript(v)
	require.NoError(t, err)
	script := v.Estack().Pop().Value()
	require.Equal(t, contractState.Script, script)
}

func TestContractIsPayable(t *testing.T) {
	v, contractState, context := createVMAndContractState(t)
	v.Estack().PushVal(vm.NewInteropItem(contractState))

	err := context.contractIsPayable(v)
	require.NoError(t, err)
	isPayable := v.Estack().Pop().Value()
	require.Equal(t, contractState.IsPayable(), isPayable)
}

func TestAssetGetAdmin(t *testing.T) {
	v, assetState, context := createVMAndAssetState(t)
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := context.assetGetAdmin(v)
	require.NoError(t, err)
	admin := v.Estack().Pop().Value()
	require.Equal(t, assetState.Admin.Bytes(), admin)
}

func TestAssetGetAmount(t *testing.T) {
	v, assetState, context := createVMAndAssetState(t)
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := context.assetGetAmount(v)
	require.NoError(t, err)
	amount := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(assetState.Amount)), amount)
}

func TestAssetGetAssetID(t *testing.T) {
	v, assetState, context := createVMAndAssetState(t)
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := context.assetGetAssetID(v)
	require.NoError(t, err)
	assetID := v.Estack().Pop().Value()
	require.Equal(t, assetState.ID.Bytes(), assetID)
}

func TestAssetGetAssetType(t *testing.T) {
	v, assetState, context := createVMAndAssetState(t)
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := context.assetGetAssetType(v)
	require.NoError(t, err)
	assetType := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(assetState.AssetType)), assetType)
}

func TestAssetGetAvailable(t *testing.T) {
	v, assetState, context := createVMAndAssetState(t)
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := context.assetGetAvailable(v)
	require.NoError(t, err)
	available := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(assetState.Available)), available)
}

func TestAssetGetIssuer(t *testing.T) {
	v, assetState, context := createVMAndAssetState(t)
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := context.assetGetIssuer(v)
	require.NoError(t, err)
	issuer := v.Estack().Pop().Value()
	require.Equal(t, assetState.Issuer.Bytes(), issuer)
}

func TestAssetGetOwner(t *testing.T) {
	v, assetState, context := createVMAndAssetState(t)
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := context.assetGetOwner(v)
	require.NoError(t, err)
	owner := v.Estack().Pop().Value()
	require.Equal(t, assetState.Owner.Bytes(), owner)
}

func TestAssetGetPrecision(t *testing.T) {
	v, assetState, context := createVMAndAssetState(t)
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := context.assetGetPrecision(v)
	require.NoError(t, err)
	precision := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(assetState.Precision)), precision)
}

// Helper functions to create VM, InteropContext, TX, AccountState, ContractState, AssetState.

func createVMAndPushBlock(t *testing.T) (*vm.VM, *Block, *interopContext) {
	v := vm.New()
	block := newDumbBlock()
	context := newInteropContext(trigger.Application, newTestChain(t), storage.NewMemoryStore(), block, nil)
	v.Estack().PushVal(vm.NewInteropItem(block))
	return v, block, context
}

func createVMAndPushTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interopContext) {
	v, tx, context := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx))
	return v, tx, context
}

func createVMAndAssetState(t *testing.T) (*vm.VM, *AssetState, *interopContext) {
	v := vm.New()
	assetState := &AssetState{
		ID:         util.Uint256{},
		AssetType:  transaction.GoverningToken,
		Name:       "TestAsset",
		Amount:     1,
		Available:  2,
		Precision:  1,
		FeeMode:    1,
		FeeAddress: randomUint160(),
		Owner:      &keys.PublicKey{X: big.NewInt(1), Y: big.NewInt(1)},
		Admin:      randomUint160(),
		Issuer:     randomUint160(),
		Expiration: 10,
		IsFrozen:   false,
	}

	context := newInteropContext(trigger.Application, newTestChain(t), storage.NewMemoryStore(), nil, nil)
	return v, assetState, context
}

func createVMAndContractState(t *testing.T) (*vm.VM, *ContractState, *interopContext) {
	v := vm.New()
	contractState := &ContractState{
		Script:      []byte("testscript"),
		ParamList:   []smartcontract.ParamType{smartcontract.StringType, smartcontract.IntegerType, smartcontract.Hash160Type},
		ReturnType:  smartcontract.ArrayType,
		Properties:  smartcontract.HasStorage,
		Name:        randomString(10),
		CodeVersion: randomString(10),
		Author:      randomString(10),
		Email:       randomString(10),
		Description: randomString(10),
		scriptHash:  randomUint160(),
	}

	context := newInteropContext(trigger.Application, newTestChain(t), storage.NewMemoryStore(), nil, nil)
	return v, contractState, context
}

func createVMAndAccState(t *testing.T) (*vm.VM, *AccountState, *interopContext) {
	v := vm.New()
	rawHash := "4d3b96ae1bcc5a585e075e3b81920210dec16302"
	hash, err := util.Uint160DecodeString(rawHash)
	accountState := NewAccountState(hash)

	key := &keys.PublicKey{X: big.NewInt(1), Y: big.NewInt(1)}
	accountState.Votes = []*keys.PublicKey{key}

	require.NoError(t, err)
	context := newInteropContext(trigger.Application, newTestChain(t), storage.NewMemoryStore(), nil, nil)
	return v, accountState, context
}

func createVMAndTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interopContext) {
	v := vm.New()
	tx := newMinerTX()

	bytes := make([]byte, 1)
	attributes := append(tx.Attributes, &transaction.Attribute{
		Usage: transaction.Description,
		Data:  bytes,
	})

	inputs := append(tx.Inputs, &transaction.Input{
		PrevHash:  randomUint256(),
		PrevIndex: 1,
	})

	outputs := append(tx.Outputs, &transaction.Output{
		AssetID:    randomUint256(),
		Amount:     10,
		ScriptHash: randomUint160(),
		Position:   1,
	})

	tx.Attributes = attributes
	tx.Inputs = inputs
	tx.Outputs = outputs
	context := newInteropContext(trigger.Application, newTestChain(t), storage.NewMemoryStore(), nil, tx)
	return v, tx, context
}