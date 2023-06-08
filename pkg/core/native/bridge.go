package native

import (
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/dao"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/mpt"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativeids"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/nativenames"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/native/noderoles"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/state"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/transaction"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/response/result"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/util/slice"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
)

const (
	MainRoleManagementId            = int32(-8)
	MainBrdgeConractDepositPrefix   = 0x01
	MainBridgeContractValidatorsKey = 0x03
	MainStateValidatorRoleValue     = 0x04

	PUSHINT8                         = 0
	PUSHINT16                        = 1
	PUSH0                            = 16
	PUSH1                            = 17
	PUSH16                           = 32
	PUSHDATA1                        = 12
	SYSCALL                          = 65
	SystemCryptoCheckSig      uint32 = 666101590
	SystemCryptoCheckMultisig uint32 = 987549854
)

const (
	PrefixHeader                     = 0x00
	PrefixStateRoot                  = 0x01
	ValidatorsKey                    = 0x02
	PrefixMainStateValidatorsAddress = 0x03
	PrefixDepositId                  = 0x04
	LockIdKey                        = 0x05
	PrefixLock                       = 0x06

	JointHeadersKey    = 0x11
	JointStateRootsKey = 0x12
)

var (
	ErrInvalidSignature           = errors.New("invalid signature")
	ErrInvalidHeader              = errors.New("invalid header")
	ErrInvalidStateRoot           = errors.New("invalid state root")
	ErrHeaderNotFound             = errors.New("header not found")
	ErrStateRootNotFound          = errors.New("state root not found")
	ErrTxInexistent               = errors.New("tx inexistent")
	ErrAlreadySynced              = errors.New("already synced")
	ErrAlreadyMinted              = errors.New("already minted")
	ErrInvalidMPTProof            = errors.New("invalid mpt proof")
	ErrTxIdNotMatchDepositedState = errors.New("txid and deposited state unmatch")
	ErrValidatorsOutdated         = errors.New("synced validators outdated")
	ErrInvalidMainValidatorsState = errors.New("invalid main validators state")
	ErrUnreachThreshold           = errors.New("mint amount unreach threshold")

	BridgeAddress  common.Address = common.Address(common.BytesToAddress([]byte{nativeids.Bridge}))
	MintThreashold                = big.NewInt(100000000) //1GAS
	BaseBonous                    = big.NewInt(3000000)   //0.03GAS
	_10GWei                       = big.NewInt(10000000000)
)

type Bridge struct {
	state.NativeContract
	cs                                   *Contracts
	standbyValidators                    keys.PublicKeys
	mainStandbyStateValidatorsScriptHash common.Address
	mainBridgeContractId                 int32
	mainNetwork                          uint32
}

func NewBridge(cs *Contracts, cfg config.ProtocolConfiguration) *Bridge {
	d := &Bridge{
		NativeContract: state.NativeContract{
			Name: nativenames.Bridge,
			Contract: state.Contract{
				Address:  BridgeAddress,
				CodeHash: hash.Keccak256(BridgeAddress[:]),
				Code:     BridgeAddress[:],
			},
		},
		cs:                                   cs,
		standbyValidators:                    cfg.StandbyValidators,
		mainStandbyStateValidatorsScriptHash: common.HexToAddress(cfg.MainStandbyStateValidatorsScriptHash),
		mainBridgeContractId:                 cfg.BridgeContractId,
		mainNetwork:                          cfg.MainNetwork,
	}
	bridgeAbi, contractCalls, err := constructAbi(d)
	if err != nil {
		panic(err)
	}
	d.Abi = *bridgeAbi
	d.ContractCalls = contractCalls
	return d
}

func createHeaderKey(index uint32) []byte {
	return makeIndexKey(PrefixHeader, index)
}

func createStateValidatorsAddressKey(index uint32) []byte {
	return makeIndexKey(PrefixMainStateValidatorsAddress, index)
}

func createStateRootKey(index uint32) []byte {
	return makeIndexKey(PrefixStateRoot, index)
}

func createDepositIdKey(depositId []byte) []byte {
	return append([]byte{PrefixDepositId}, depositId...)
}

func createLockKey(lockId []byte) []byte {
	return append([]byte{PrefixLock}, lockId...)
}

func (b *Bridge) ContractCall_syncHeader(ic InteropContext, rawHeader []byte) error {
	header := new(block.Header)
	err := io.FromByteArray(header, rawHeader)
	if err != nil {
		return err
	}
	h := b.getHeaderByIndex(ic.Dao(), header.Index)
	if h != nil {
		return ErrAlreadySynced
	}
	joint := b.lastJoint(ic.Dao(), JointHeadersKey, header.Index)
	lastJoint := b.getHeaderByIndex(ic.Dao(), joint)
	signer := hash.Hash160(header.Witness.VerificationScript)
	if lastJoint == nil && header.Index != 0 {
		return errors.New("genesis block unsynced")
	}
	if lastJoint != nil && (signer != lastJoint.NextConsensus || !b.verifyMainWitness(header, &header.Witness)) {
		return ErrInvalidSignature
	}
	b.saveHeader(ic.Dao(), header)
	if header.Index == 0 || header.NextConsensus != lastJoint.NextConsensus {
		b.addJoint(ic.Dao(), JointHeadersKey, header.Index)
	}
	return nil
}

func (b *Bridge) ContractCall_syncStateRoot(ic InteropContext, rawStateroot []byte) error {
	stateroot := new(state.MPTRoot)
	err := io.FromByteArray(stateroot, rawStateroot)
	if err != nil {
		return err
	}
	s := b.getStateRootByIndex(ic.Dao(), stateroot.Index)
	if s != nil {
		return ErrAlreadySynced
	}
	joint := b.lastJoint(ic.Dao(), JointStateRootsKey, stateroot.Index)
	validatorsAddress := b.getStateRootValidatorsAddressByIndex(ic.Dao(), joint)
	signer := hash.Hash160(stateroot.Witness.VerificationScript)
	if (validatorsAddress == (common.Address{}) && signer != b.mainStandbyStateValidatorsScriptHash) ||
		(validatorsAddress != (common.Address{}) && signer != validatorsAddress) {
		return ErrInvalidStateRoot
	}
	if !b.verifyMainWitness(stateroot, &stateroot.Witness) {
		return ErrInvalidStateRoot
	}
	b.saveStateRoot(ic.Dao(), stateroot)
	return nil
}

func (b *Bridge) ContractCall_syncStateRootValidatorsAddress(
	ic InteropContext,
	headerIndex uint32,
	txid *big.Int,
	txProof []byte,
	stateIndex uint32,
	stateProof []byte) error {
	address := b.getStateRootValidatorsAddressByIndex(ic.Dao(), headerIndex)
	if address != (common.Address{}) {
		return ErrAlreadySynced
	}
	txHash := common.BytesToHash(txid.Bytes())
	key, value, err := b.verifyState(ic.Dao(), headerIndex, txHash, txProof, stateIndex, stateProof)
	if err != nil {
		return err
	}
	ok, dindex := isDesignateStateValidators(key)
	if !ok || dindex != headerIndex+1 {
		return ErrInvalidMPTProof
	}
	pks, err := parseMainInteropListECPoints(value)
	if err != nil {
		return fmt.Errorf("can't decode public keys:%w", err)
	}
	address = hash.Hash160(createMainDefaultMultiSigRedeemScript(pks))
	lastJoint := b.lastJoint(ic.Dao(), JointStateRootsKey, headerIndex)
	jointAddress := b.getStateRootValidatorsAddressByIndex(ic.Dao(), lastJoint)
	if jointAddress == address {
		return errors.New("not joint state root address")
	}
	b.saveStateRootValidatorsAddress(ic.Dao(), dindex, address)
	return b.addJoint(ic.Dao(), JointStateRootsKey, dindex)
}

func (b *Bridge) ContractCall_syncValidators(
	ic InteropContext,
	headerIndex uint32,
	txid *big.Int,
	txProof []byte,
	stateIndex uint32,
	stateProof []byte) error {
	syncedIndex := b.getValidatorsSyncedIndex(ic.Dao())
	if syncedIndex > 0 && syncedIndex >= headerIndex {
		return ErrValidatorsOutdated
	}
	txHash := common.BytesToHash(txid.Bytes())
	key, value, err := b.verifyState(ic.Dao(), headerIndex, txHash, txProof, stateIndex, stateProof)
	if err != nil {
		return err
	}
	if !b.isDesignateValidators(key) {
		return errors.New("not designate validators proof")
	}
	mstate := new(mainValidatorState)
	err = mstate.DecodeBytes(value)
	if err != nil {
		return err
	}
	if mstate.txid != txHash {
		return errors.New("invalid main validators state")
	}
	sort.Sort(mstate.pks)
	b.saveValidatorsSyncedIndex(ic, headerIndex)
	b.cs.Designate.designateAsRole(ic, noderoles.Validator, mstate.pks)
	b.cs.Designate.designateAsRole(ic, noderoles.StateValidator, mstate.pks)
	return nil
}

func (b *Bridge) ContractCall_requestMint(
	ic InteropContext,
	headerIndex uint32,
	txid *big.Int,
	txProof []byte,
	stateIndex uint32,
	stateProof []byte) error {
	txHash := common.BytesToHash(txid.Bytes())
	key, value, err := b.verifyState(ic.Dao(), headerIndex, txHash, txProof, stateIndex, stateProof)
	if err != nil {
		return err
	}
	if !b.isMintRequest(key) {
		return ErrInvalidMPTProof
	}
	depositId := key[5:]
	minted := b.getMintedState(ic.Dao(), depositId)
	if minted != nil {
		return ErrAlreadyMinted
	}
	ds, err := newDepositStateFromBytes(value)
	if err != nil {
		return fmt.Errorf("invalid deposited state: %w", err)
	}
	if ds.txId != txHash {
		return ErrTxIdNotMatchDepositedState
	}
	amount := big.NewInt(int64(ds.amount))
	if amount.Cmp(MintThreashold) < 0 {
		return ErrUnreachThreshold
	}
	mintAmount := big.NewInt(0).Sub(amount, BaseBonous)
	err = b.cs.GAS.Mint(ic.Dao(), ds.to, big.NewInt(0).Mul(mintAmount, _10GWei))
	if err != nil {
		return err
	}
	b.saveMintedState(ic, depositId, txHash)
	err = b.mintBonous(ic, BaseBonous)
	if err != nil {
		return err
	}
	log(ic, b.Address, mintAmount.Bytes(), txHash, common.BytesToHash(ds.to[:]))
	return nil
}

func (b *Bridge) ContractCall__View_getMinted(ic InteropContext, depositId int64) ([]byte, error) {
	txid, err := b.GetMinted(ic.Dao(), depositId)
	if err != nil {
		return nil, err
	}
	return txid[:], nil
}

func (b *Bridge) GetMinted(d *dao.Simple, depositId int64) (common.Hash, error) {
	idBytes := big.NewInt(int64(depositId)).Bytes()
	state := b.getMintedState(d, idBytes)
	if state == nil {
		return common.Hash{}, nil
	}
	return state.mintTx, nil
}

func (b *Bridge) newLockId(d *dao.Simple) []byte {
	id := make([]byte, 8)
	num := uint64(0)
	oldId := d.GetStorageItem(b.Address, []byte{LockIdKey})
	if len(oldId) > 0 {
		num = binary.LittleEndian.Uint64(oldId) + 1
	}
	binary.LittleEndian.PutUint64(id, num)
	d.PutStorageItem(b.Address, []byte{LockIdKey}, id)
	return id
}

func (b *Bridge) ContractCall_Payable_lock(ic InteropContext, to common.Address) error {
	value := ic.Container().Value()
	value = big.NewInt(0).Div(value, _10GWei)
	if value.Cmp(MintThreashold) < 0 {
		return ErrUnreachThreshold
	}
	from := ic.Container().From()
	state := depositState{
		txId:   ic.Container().Hash(),
		from:   from,
		to:     to,
		amount: value.Uint64(),
	}
	ic.Dao().PutStorageItem(b.Address, createLockKey(b.newLockId(ic.Dao())), state.Bytes())
	b.cs.GAS.Burn(ic.Dao(), from, value)
	log(ic, from, value.Bytes(), b.Abi.Events["lock"].ID, to.Hash())
	return nil
}

func (b *Bridge) verifyState(
	d *dao.Simple,
	headerIndex uint32,
	txid common.Hash,
	txProof []byte,
	stateIndex uint32,
	stateProof []byte) ([]byte, []byte, error) {
	header := b.getHeaderByIndex(d, headerIndex)
	if header == nil {
		return nil, nil, ErrHeaderNotFound
	}
	if stateIndex < headerIndex {
		return nil, nil, ErrInvalidStateRoot
	}
	stateroot := b.getStateRootByIndex(d, stateIndex)
	if stateroot == nil {
		return nil, nil, ErrStateRootNotFound
	}
	if !b.verifyMerkleProof(header.MerkleRoot, txid, txProof) {
		return nil, nil, ErrTxInexistent
	}
	return verifyMPTProof(stateroot.Root, stateProof)
}

func (b *Bridge) getHeaderByIndex(d *dao.Simple, index uint32) *block.Header {
	key := createHeaderKey(index)
	bs := d.GetStorageItem(b.Address, key)
	if len(bs) == 0 {
		return nil
	}
	header := new(block.Header)
	err := io.FromByteArray(header, bs)
	if err != nil {
		panic(fmt.Errorf("can't parse header: %w", err))
	}
	return header
}

func (b *Bridge) getStateRootByIndex(d *dao.Simple, index uint32) *state.MPTRoot {
	key := createStateRootKey(index)
	bs := d.GetStorageItem(b.Address, key)
	if len(bs) == 0 {
		return nil
	}
	sr := new(state.MPTRoot)
	err := io.FromByteArray(sr, bs)
	if err != nil {
		panic(fmt.Errorf("can't parse state root: %w", err))
	}
	return sr
}

func (b *Bridge) getStateRootValidatorsAddressByIndex(d *dao.Simple, index uint32) common.Address {
	key := createStateValidatorsAddressKey(index)
	bs := d.GetStorageItem(b.Address, key)
	if len(bs) == 0 {
		return common.Address{}
	}
	return common.BytesToAddress(bs)
}

func (b *Bridge) getMintedState(d *dao.Simple, depositId []byte) *mintedState {
	key := createDepositIdKey(depositId)
	raw := d.GetStorageItem(b.Address, key)
	if raw == nil {
		return nil
	}
	ms, err := newMintedStateFromBytes(raw)
	if err != nil {
		panic(err)
	}
	return ms
}

func (b *Bridge) getValidatorsSyncedIndex(d *dao.Simple) uint32 {
	key := []byte{ValidatorsKey}
	bs := d.GetStorageItem(b.Address, key)
	if len(bs) == 0 {
		return 0
	}
	return binary.LittleEndian.Uint32(bs)
}

func (b *Bridge) saveHeader(d *dao.Simple, header *block.Header) {
	key := createHeaderKey(header.Index)
	bs, err := io.ToByteArray(header)
	if err != nil {
		panic(err)
	}
	d.PutStorageItem(b.Address, key, bs)
}

func (b *Bridge) saveStateRootValidatorsAddress(d *dao.Simple, index uint32, address common.Address) {
	key := createStateValidatorsAddressKey(index)
	d.PutStorageItem(b.Address, key, address[:])
}

func (b *Bridge) saveStateRoot(d *dao.Simple, stateroot *state.MPTRoot) {
	key := createStateRootKey(stateroot.Index)
	bs, err := io.ToByteArray(stateroot)
	if err != nil {
		panic(err)
	}
	d.PutStorageItem(b.Address, key, bs)
}

func (b *Bridge) saveValidatorsSyncedIndex(ic InteropContext, index uint32) {
	key := []byte{ValidatorsKey}
	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, index)
	ic.Dao().PutStorageItem(b.Address, key, value)
}

func (b *Bridge) saveMintedState(ic InteropContext, depositId []byte, txid common.Hash) {
	key := createDepositIdKey(depositId)
	mintedState := mintedState{
		depositTx: txid,
		mintTx:    ic.Container().Hash(),
	}
	ic.Dao().PutStorageItem(b.Address, key, mintedState.Bytes())
}

func (b *Bridge) lastJoint(d *dao.Simple, key byte, index uint32) uint32 {
	joints := b.joints(d, []byte{key})
	if len(joints) == 0 {
		return 0
	}
	for i := len(joints) - 1; i > 0; i++ {
		if joints[i] <= index {
			return joints[i]
		}
	}
	return 0
}

func (b *Bridge) joints(d *dao.Simple, key []byte) []uint32 {
	data := d.GetStorageItem(b.Address, key)
	if len(data) == 0 {
		return nil
	}
	if len(data)%4 != 0 {
		panic("internal error: invalid data length in joint headers")
	}
	r := make([]uint32, len(data)/4)
	for i := 0; i*4 < len(data); i++ {
		r[i] = binary.LittleEndian.Uint32(data[i*4 : 4*(i+1)])
	}
	return r
}

func (b *Bridge) addJoint(d *dao.Simple, key byte, index uint32) error {
	joints := b.joints(d, []byte{key})
	var newJoints []uint32
	if len(joints) == 0 {
		newJoints = []uint32{index}
	} else {
		newJoints = make([]uint32, len(joints)+1)
		for i := len(joints) - 1; i > 0; i++ {
			if index > joints[i] {
				newJoints = append(append(joints[:i+1], index), joints[i+1:]...)
				break
			}
		}
	}
	b.saveJoints(d, key, newJoints)
	return nil
}

func (b *Bridge) saveJoints(d *dao.Simple, key byte, joints []uint32) {
	bs := make([]byte, len(joints)*4)
	for i, v := range joints {
		binary.LittleEndian.PutUint32(bs[i*4:], v)
	}
	d.PutStorageItem(b.Address, []byte{key}, bs)
}

func (b *Bridge) verifyMainWitness(hh hash.Hashable, w *transaction.Witness) bool {
	isSingle, pk := isMainSingleSignature(w.VerificationScript)
	if isSingle {
		return pk.Verify(w.InvocationScript[2:], netSha256(b.mainNetwork, hh))
	}
	isMulti, m, n, pks := isMainMultiSignature(w.VerificationScript)
	if isMulti {
		msg := netSha256(b.mainNetwork, hh)
		signatures := getMainMultiSignatures(w.InvocationScript)
		for x, y := 0, 0; x < m && y < n; {
			if pks[y].Verify(signatures[x], msg) {
				x++
			}
			y++
			if m-x > n-y {
				return false
			}
		}
		return true
	}
	return false
}

func (b *Bridge) verifyMerkleProof(root common.Hash, txid common.Hash, proof []byte) bool {
	if len(proof) < 4 {
		return false
	}
	path := binary.LittleEndian.Uint32(proof[:4])
	if (len(proof)-4)%common.HashLength != 0 {
		return false
	}
	count := len(proof) / common.HashLength
	hashes := make([]common.Hash, count)
	for i := 0; i < count; i++ {
		hashes[i] = common.BytesToHash(proof[4+i*common.HashLength : 4+(i+1)*common.HashLength])
	}
	return hash.VerifyMerkleProof(root, txid, hashes, path)
}

func contractId(key []byte) int32 {
	return int32(binary.LittleEndian.Uint32(key))
}

func isDesignateStateValidators(key []byte) (bool, uint32) {
	if len(key) != 9 {
		return false, 0
	}
	if contractId(key[:4]) != MainRoleManagementId {
		return false, 0
	}
	if key[4] != MainStateValidatorRoleValue {
		return false, 0
	}
	return true, binary.BigEndian.Uint32(key[5:])
}

func (b *Bridge) isBridgeContract(key []byte) bool {
	return contractId(key) == b.mainBridgeContractId
}

func (b *Bridge) isDesignateValidators(key []byte) bool {
	if len(key) != 5 {
		return false
	}
	if !b.isBridgeContract(key[:4]) {
		return false
	}
	return key[4] == MainBridgeContractValidatorsKey
}

func (b *Bridge) isMintRequest(key []byte) bool {
	if len(key) < 4 {
		return false
	}
	if !b.isBridgeContract(key[:4]) {
		return false
	}
	return key[4] == MainBrdgeConractDepositPrefix
}

func (b *Bridge) mintBonous(ic InteropContext, bonous *big.Int) error {
	return b.cs.GAS.Mint(ic.Dao(), ic.Sender(), big.NewInt(0).Mul(bonous, _10GWei))
}

func (d *Bridge) RequiredGas(ic InteropContext, input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := d.Abi.MethodById(input[:4])
	if err != nil {
		return 0
	}
	switch method.Name {
	case "initialize":
		return 0
	case "getMinted":
		return defaultNativeReadFee
	case "syncHeader", "syncStateRoot", "syncStateRootValidatorsAddress", "syncValidators", "requestMint":
		return defaultNativeWriteFee
	default:
		return 0
	}
}

func (d *Bridge) Run(ic InteropContext, input []byte) ([]byte, error) {
	return contractCall(d, &d.NativeContract, ic, input)
}

type mainValidatorState struct {
	txid common.Hash
	pks  keys.PublicKeys
}

func (v *mainValidatorState) DecodeBytes(b []byte) error {
	if len(b) < common.HashLength+1 {
		return errors.New("invalid main validators state")
	}
	v.txid = common.BytesToHash(b[:32])
	offset := 32
	count := int(b[offset])
	offset++
	v.pks = keys.PublicKeys{}
	for i := 0; i < count; i++ {
		if offset > len(b) {
			return ErrInvalidMainValidatorsState
		}
		len := int(b[offset])
		offset++
		pk, err := keys.NewPublicKeyFromBytes(b[offset:(offset+len)], btcec.S256())
		if err != nil {
			return errors.New("can't parse public key")
		}
		v.pks = append(v.pks, pk)
		offset += len
	}
	return nil
}

type depositState struct {
	txId   common.Hash
	from   common.Address
	amount uint64
	to     common.Address
}

func newDepositStateFromBytes(b []byte) (*depositState, error) {
	d := new(depositState)
	reader := io.NewBinReaderFromBuf(b)
	reader.ReadBytes(d.txId[:])
	reader.ReadBytes(d.from[:])
	d.amount = reader.ReadU64LE()
	reader.ReadBytes(d.to[:])
	slice.Reverse(d.to[:])
	return d, reader.Err
}

func (d *depositState) Bytes() []byte {
	w := io.NewBufBinWriter()
	w.WriteBytes(d.txId[:])
	w.WriteBytes(d.from[:])
	w.WriteU64LE(d.amount)
	w.WriteBytes(d.to[:])
	return w.Bytes()
}

type mintedState struct {
	depositTx common.Hash
	mintTx    common.Hash
}

func newMintedStateFromBytes(b []byte) (*mintedState, error) {
	d := new(mintedState)
	if len(b) != 2*common.HashLength {
		return nil, errors.New("invalid minted state bytes")
	}
	d.depositTx = common.BytesToHash(b[:common.HashLength])
	d.mintTx = common.BytesToHash(b[common.HashLength:])
	return d, nil
}

func (d *mintedState) Bytes() []byte {
	return append(d.depositTx[:], d.mintTx[:]...)
}

func createMainDefaultMultiSigRedeemScript(pks keys.PublicKeys) []byte {
	i := 0
	script := make([]byte, 9+35*pks.Len())
	m := keys.GetDefaultHonestNodeCount(pks.Len())
	script[i] = PUSHINT8
	i++
	script[i] = byte(m)
	i++
	for _, pk := range pks {
		script[i] = PUSHDATA1
		i++
		script[i] = 33
		i++
		copy(script[i:], pk.Bytes())
		i += 33
	}
	script[i] = PUSHINT8
	i++
	script[i] = byte(len(pks))
	i++
	script[i] = SYSCALL
	i++
	binary.LittleEndian.PutUint32(script[i:], SystemCryptoCheckMultisig)
	return script
}

func isMainSingleSignature(script []byte) (bool, *keys.PublicKey) {
	if len(script) != 40 {
		return false, nil
	}
	if script[0] != PUSHDATA1 || script[1] != 33 || script[35] != SYSCALL || binary.LittleEndian.Uint32(script[36:]) != SystemCryptoCheckSig {
		return false, nil
	}
	pk, err := keys.NewPublicKeyFromBytes(script[2:35], elliptic.P256())
	if err != nil {
		return false, nil
	}
	return true, pk
}

func isMainMultiSignature(script []byte) (bool, int, int, keys.PublicKeys) {
	if len(script) < 42 {
		return false, 0, 0, nil
	}
	m, n := 0, 0
	pks := []*keys.PublicKey{}
	i := 0
	switch script[i] {
	case PUSHINT8:
		i++
		m = int(script[i])
		i++
	case PUSHINT16:
		i++
		m = int(binary.LittleEndian.Uint16(script[i:]))
		i += 2
	default:
		if script[i] <= PUSH16 && PUSH1 <= script[i] {
			m = int(script[i] - PUSH0)
			i++
		} else {
			return false, m, n, nil
		}
	}
	if m < 1 || 1024 < m {
		return false, m, n, nil
	}
	for script[i] == PUSHDATA1 {
		if len(script) <= i+35 || script[i+1] != 33 {
			return false, m, n, nil
		}
		i += 2
		pk, err := keys.NewPublicKeyFromBytes(script[i:i+33], elliptic.P256())
		if err != nil {
			return false, m, n, nil
		}
		pks = append(pks, pk)
		i += 33
		n++
	}
	if n < m || 1024 < n {
		return false, m, n, nil
	}
	switch script[i] {
	case PUSHINT8:
		if len(script) <= i+1 || n != int(script[i+1]) {
			return false, m, n, nil
		}
		i += 2
	case PUSHINT16:
		if len(script) <= i+3 || n != int(binary.LittleEndian.Uint16(script[i+1:])) {
			return false, m, n, nil
		}
		i += 3
	default:
		if script[i] <= PUSH16 && PUSH1 <= script[i] {
			if n != int(script[i]-PUSH0) {
				return false, m, n, nil
			}
			i++
		} else {
			return false, m, n, nil
		}
	}
	if len(script) != i+5 || script[i] != SYSCALL || binary.LittleEndian.Uint32(script[i+1:]) != SystemCryptoCheckMultisig {
		return false, m, n, nil
	}
	return true, m, n, keys.PublicKeys(pks)
}

func getMainMultiSignatures(script []byte) [][]byte {
	signatures := [][]byte{}
	for i := 0; i < len(script); {
		if script[i] != PUSHDATA1 {
			return nil
		}
		i++
		if i+65 > len(script) {
			return nil
		}
		if script[i] != 64 {
			return nil
		}
		i++
		signatures = append(signatures, script[i:i+64])
		i += 64
	}
	return signatures
}

func netSha256(network uint32, hh hash.Hashable) []byte {
	return hash.Sha256(getMainSignedData(network, hh)).Bytes()
}

func getMainSignedData(network uint32, hh hash.Hashable) []byte {
	var b = make([]byte, 4+32)
	binary.LittleEndian.PutUint32(b, network)
	h := hh.Hash()
	copy(b[4:], h[:])
	return b
}

func verifyMPTProof(root common.Hash, proof []byte) (key []byte, value []byte, err error) {
	pwk := new(result.ProofWithKey)
	err = io.FromByteArray(pwk, proof)
	if err != nil {
		return
	}
	value, ok := mpt.VerifyProof(root, pwk.Key, pwk.Proof)
	if !ok {
		err = ErrInvalidMPTProof
		return
	}
	return pwk.Key, value, nil
}

func parseMainInteropListECPoints(data []byte) (keys.PublicKeys, error) {
	var (
		StackItemArray      byte = 64
		StackItemByteString byte = 40
		ErrEOF                   = errors.New("unexpect EOF")
		ErrFormat                = errors.New("invalid format")
	)
	offset := 0
	if len(data) <= offset {
		return nil, ErrEOF
	}
	if data[offset] != StackItemArray {
		return nil, ErrFormat
	}
	offset++
	if len(data) <= offset {
		return nil, ErrEOF
	}
	count := int(data[offset])
	pubs := make([]*keys.PublicKey, count)
	offset++
	for i := 0; i < count; i++ {
		if len(data) <= offset {
			return nil, ErrEOF
		}
		if data[offset] != StackItemByteString {
			return nil, ErrFormat
		}
		offset++
		if len(data) <= offset {
			return nil, ErrEOF
		}
		le := int(data[offset])
		offset++
		if len(data) < offset+le {
			return nil, ErrEOF
		}
		p, err := keys.NewPublicKeyFromBytes(data[offset:offset+le], elliptic.P256())
		if err != nil {
			return nil, err
		}
		pubs[i] = p
		offset += le
	}
	return pubs, nil
}
