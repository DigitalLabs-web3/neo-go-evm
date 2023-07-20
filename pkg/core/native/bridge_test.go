package native

import (
	"crypto/elliptic"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/config"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/core/block"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/keys"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/util/slice"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestParseContractId(t *testing.T) {
	var a int32 = -1
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, uint32(a))
	v := int32(binary.LittleEndian.Uint32(data))
	println(v)
}
func newMainPublicKeyFromString(s string) (*keys.PublicKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return keys.NewPublicKeyFromBytes(b, elliptic.P256())
}

func mainPublicKeysFromStrings(ss []string) (keys.PublicKeys, error) {
	arr := make([]*keys.PublicKey, len(ss))
	for i := range ss {
		pubKey, err := newMainPublicKeyFromString(ss[i])
		if err != nil {
			return nil, err
		}
		arr[i] = pubKey
	}
	return keys.PublicKeys(arr), nil
}

func TestCreateMultiScript(t *testing.T) {
	pks, err := mainPublicKeysFromStrings([]string{
		"03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c",
		"02df48f60e8f3e01c48ff40b9b7f1310d7a8b2a193188befe1c2e3df740e895093",
		"03b8d9d5771d8f513aa0869b9cc8d50986403b78c6da36890638c3d46a5adce04a",
		"02ca0e27697b9c248f6f16e085fd0061e26f44da85b58ee835c110caa5ec3ba554",
		"024c7b7fb6c310fccf1ba33b082519d82964ea93868d676662d4a59ad548df0e7d",
		"02aaec38470f6aad0042c6e877cfd8087d2676b0f516fddd362801b9bd3936399e",
		"02486fd15702c4490a26703112a5cc1d0923fd697a33406bd5a1c00e0013b09a70",
	})
	assert.NoError(t, err)
	script := createMainDefaultMultiSigRedeemScript(pks)
	assert.Equal(t, 9+35*7, len(script))
}

func TestIsMainMultiSig(t *testing.T) {
	case1 := []byte{
		0, 2, 12, 33, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221,
		221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 221, 12, 33, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 0,
	}
	ok, _, _, _ := isMainMultiSignature(case1)
	assert.False(t, ok)
	case2 := []byte{
		18, 12, 33, 2, 111, 240, 59, 148, 146, 65, 206, 29, 173, 212, 53, 25, 230, 150, 14, 10, 133, 180, 26,
		105, 160, 92, 50, 129, 3, 170, 43, 206, 21, 148, 202, 22, 12, 33, 2, 111, 240, 59, 148, 146, 65, 206,
		29, 173, 212, 53, 25, 230, 150, 14, 10, 133, 180, 26, 105, 160, 92, 50, 129, 3, 170, 43, 206, 21, 148,
		202, 22, 18,
	}
	ok, _, _, _ = isMainMultiSignature(case2)
	assert.False(t, ok)

	pks, err := mainPublicKeysFromStrings([]string{
		"03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c",
		"02df48f60e8f3e01c48ff40b9b7f1310d7a8b2a193188befe1c2e3df740e895093",
		"03b8d9d5771d8f513aa0869b9cc8d50986403b78c6da36890638c3d46a5adce04a",
		"02ca0e27697b9c248f6f16e085fd0061e26f44da85b58ee835c110caa5ec3ba554",
		"024c7b7fb6c310fccf1ba33b082519d82964ea93868d676662d4a59ad548df0e7d",
		"02aaec38470f6aad0042c6e877cfd8087d2676b0f516fddd362801b9bd3936399e",
		"02486fd15702c4490a26703112a5cc1d0923fd697a33406bd5a1c00e0013b09a70",
	})
	assert.NoError(t, err)
	script := createMainDefaultMultiSigRedeemScript(pks)
	ok, m, n, _ := isMainMultiSignature(script)
	assert.True(t, ok)
	assert.Equal(t, 5, m)
	assert.Equal(t, 7, n)
}

func TestNewBridge(t *testing.T) {
	conn := NewBridge(nil, config.ProtocolConfiguration{})
	_, err := conn.Abi.Pack("syncValidators", uint32(0), big.NewInt(0).SetBytes(common.BytesToHash([]byte{1}).Bytes()), []byte{2}, uint32(0), []byte{3})
	assert.NoError(t, err)
}

func TestVerifyMPTProof(t *testing.T) {
	proof, err := hex.DecodeString("09f8ffffff04000005c8045200035f15db83a36428c56d53c7dd516eb0f452d8b49c17d87741d40c823a346249ab040404040404040404040404040403bd9ff52190bfa98b38aa2db58f1a69151d330378a76ab15c759978b4c34588ea04d20004040404040404031efa3945cf6ff964a97ec9cc5ccef992ef998ef6cb2615979ff961e8bfb5c81d03886bbbc3d73fc4589f8fe21c99bee2d7cadd571f214e8b015868db1d1a69acc203893eb31e1fec8826747366df044ca63e644d4e25d200a1486a69a39a79c19c9603e331b1fb628175e32706c04f570c2f13be26fe47701c9c950918f6fca2498845032ba9de06cf7eb77a474c698015bbea1c00ce79e8440017476168513bd544d87304040403d9db51d9800dde29c213241a738cd7fd1ffef2cceb7c273bcef67a6274d2095b043301100f0f0f0f0f0f00040000000000050c08037279f38b14e3fdbba53bad6ea90ba4b6c1ddbfe12e5def423e1f70c26536a34927022540012821036935e573d509f195523f9166f9f1a154677db23d128a7387b9c4f5c5467ce6d0")
	assert.NoError(t, err)
	key, val, err := verifyMPTProof(common.HexToHash("0xab13fb9a1731514204fa14cbd07fe19fa8a09c5916ac2b4650fcb13fddacb572"), proof)
	assert.NoError(t, err)
	fmt.Println(hex.EncodeToString(key))
	fmt.Println(hex.EncodeToString(val))
}

func TestStateValidatorDesignateKey(t *testing.T) {
	b, _ := hex.DecodeString("f8ffffff04000005c8")
	ok, _ := isDesignateStateValidators(b)
	assert.True(t, ok)
}

func TestMainStateValidatorsList(t *testing.T) {
	b, _ := hex.DecodeString("40022821036935e573d509f195523f9166f9f1a154677db23d128a7387b9c4f5c5467ce6d02821036935e573d509f195523f9166f9f1a154677db23d128a7387b9c4f5c5467ce6d0")
	_, err := parseMainInteropListECPoints(b)
	assert.NoError(t, err)
}

func TestMainDepositState(t *testing.T) {
	b, err := hex.DecodeString("11196ceb1a4bdceeb79f79c9a498cd23e398256910a3913db1b4d9b353085c526d6f8e50365673babf21b56d36bd9b7f6289ac440400e1f5056d6f8e50365673babf21b56d36bd9b7f6289ac44")
	assert.NoError(t, err)
	_, err = newDepositStateFromBytes(b)
	assert.NoError(t, err)
}

func TestMainDepositState1(t *testing.T) {
	b, err := hex.DecodeString("512e9efb16d7af73792bb5ea4461cf696ab439e6e62a35fb0b483a87d3ba5ca740cb1198c7a2b5a4f795d2ad75fb14966505a7fb0400e1f505333e813995036d37ef43af12b5566c956e4ea588")
	assert.NoError(t, err)
	d, err := newDepositStateFromBytes(b)
	t.Log(d.amount)
	assert.NoError(t, err)
}

func TestVerifyHeader(t *testing.T) {
	bridge := NewBridge(nil, config.ProtocolConfiguration{MainNetwork: 2})
	b, err := hex.DecodeString("000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000088ea19ef550100001dac2b7c000000000000000000dceb4f94ec774633125a267eec8df86510dd998701000111")
	assert.NoError(t, err)
	h := new(block.Header)
	err = io.FromByteArray(h, b)
	assert.NoError(t, err)
	assert.True(t, bridge.verifyMainWitness(h, &h.Witness))
}

func TestAddress(t *testing.T) {
	b, _ := hex.DecodeString("eE1e68ed1B7f892048D0FfEc42a72102Dd31Db0a")
	a := common.BytesToAddress(b)
	t.Log(hex.EncodeToString(a.Bytes()))
	t.Log(a)
}

func TestIsMint(t *testing.T) {
	bri := NewBridge(nil, config.ProtocolConfiguration{BridgeContractId: 1})
	key, err := hex.DecodeString("010000000101")
	assert.NoError(t, err)
	assert.True(t, bri.isMintRequest(key))
}

func TestIsDesignateStateValidators(t *testing.T) {
	key, err := hex.DecodeString("f8ffffff0400000e10")
	assert.NoError(t, err)
	ok, h := isDesignateStateValidators(key)
	assert.True(t, ok)
	t.Log(h)
}

func TestContractId(t *testing.T) {
	key, err := hex.DecodeString("f8ffffff")
	assert.NoError(t, err)
	id := contractId(key)
	assert.Equal(t, int32(-8), id)
}

func TestStandbyScriptHash(t *testing.T) {
	main := "0x8799dd1065f88dec7e265a12334677ec944febdc"
	b := common.HexToAddress(main)
	t.Log(hex.EncodeToString(b[:]))
	slice.Reverse(b[:])
	t.Log(b)
}

func TestSendBack(t *testing.T) {
	GWei := big.NewInt(1000000000)
	value := big.NewInt(0).Mul(big.NewInt(1000000001), GWei)
	amount, back := parseBurnValue(value)
	assert.Equal(t, amount, big.NewInt(100000000))
	assert.Equal(t, back, GWei)
}
