package rfc6979_test

import (
	"crypto/dsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"math/big"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/rfc6979"
)

type dsaFixture struct {
	name    string
	key     *dsaKey
	alg     func() hash.Hash
	message string
	r, s    string
}

type dsaKey struct {
	key      *dsa.PrivateKey
	subgroup int
}

var dsa1024 = &dsaKey{
	key: &dsa.PrivateKey{
		PublicKey: dsa.PublicKey{
			Parameters: dsa.Parameters{
				P: dsaLoadInt("86F5CA03DCFEB225063FF830A0C769B9DD9D6153AD91D7CE27F787C43278B447E6533B86B18BED6E8A48B784A14C252C5BE0DBF60B86D6385BD2F12FB763ED8873ABFD3F5BA2E0A8C0A59082EAC056935E529DAF7C610467899C77ADEDFC846C881870B7B19B2B58F9BE0521A17002E3BDD6B86685EE90B3D9A1B02B782B1779"),
				Q: dsaLoadInt("996F967F6C8E388D9E28D01E205FBA957A5698B1"),
				G: dsaLoadInt("07B0F92546150B62514BB771E2A0C0CE387F03BDA6C56B505209FF25FD3C133D89BBCD97E904E09114D9A7DEFDEADFC9078EA544D2E401AEECC40BB9FBBF78FD87995A10A1C27CB7789B594BA7EFB5C4326A9FE59A070E136DB77175464ADCA417BE5DCE2F40D10A46A3A3943F26AB7FD9C0398FF8C76EE0A56826A8A88F1DBD"),
			},
			Y: dsaLoadInt("5DF5E01DED31D0297E274E1691C192FE5868FEF9E19A84776454B100CF16F65392195A38B90523E2542EE61871C0440CB87C322FC4B4D2EC5E1E7EC766E1BE8D4CE935437DC11C3C8FD426338933EBFE739CB3465F4D3668C5E473508253B1E682F65CBDC4FAE93C2EA212390E54905A86E2223170B44EAA7DA5DD9FFCFB7F3B"),
		},
		X: dsaLoadInt("411602CB19A6CCC34494D79D98EF1E7ED5AF25F7"),
	},
	subgroup: 160,
}

var dsa2048 = &dsaKey{
	key: &dsa.PrivateKey{
		PublicKey: dsa.PublicKey{
			Parameters: dsa.Parameters{
				P: dsaLoadInt("9DB6FB5951B66BB6FE1E140F1D2CE5502374161FD6538DF1648218642F0B5C48C8F7A41AADFA187324B87674FA1822B00F1ECF8136943D7C55757264E5A1A44FFE012E9936E00C1D3E9310B01C7D179805D3058B2A9F4BB6F9716BFE6117C6B5B3CC4D9BE341104AD4A80AD6C94E005F4B993E14F091EB51743BF33050C38DE235567E1B34C3D6A5C0CEAA1A0F368213C3D19843D0B4B09DCB9FC72D39C8DE41F1BF14D4BB4563CA28371621CAD3324B6A2D392145BEBFAC748805236F5CA2FE92B871CD8F9C36D3292B5509CA8CAA77A2ADFC7BFD77DDA6F71125A7456FEA153E433256A2261C6A06ED3693797E7995FAD5AABBCFBE3EDA2741E375404AE25B"),
				Q: dsaLoadInt("F2C3119374CE76C9356990B465374A17F23F9ED35089BD969F61C6DDE9998C1F"),
				G: dsaLoadInt("5C7FF6B06F8F143FE8288433493E4769C4D988ACE5BE25A0E24809670716C613D7B0CEE6932F8FAA7C44D2CB24523DA53FBE4F6EC3595892D1AA58C4328A06C46A15662E7EAA703A1DECF8BBB2D05DBE2EB956C142A338661D10461C0D135472085057F3494309FFA73C611F78B32ADBB5740C361C9F35BE90997DB2014E2EF5AA61782F52ABEB8BD6432C4DD097BC5423B285DAFB60DC364E8161F4A2A35ACA3A10B1C4D203CC76A470A33AFDCBDD92959859ABD8B56E1725252D78EAC66E71BA9AE3F1DD2487199874393CD4D832186800654760E1E34C09E4D155179F9EC0DC4473F996BDCE6EED1CABED8B6F116F7AD9CF505DF0F998E34AB27514B0FFE7"),
			},
			Y: dsaLoadInt("667098C654426C78D7F8201EAC6C203EF030D43605032C2F1FA937E5237DBD949F34A0A2564FE126DC8B715C5141802CE0979C8246463C40E6B6BDAA2513FA611728716C2E4FD53BC95B89E69949D96512E873B9C8F8DFD499CC312882561ADECB31F658E934C0C197F2C4D96B05CBAD67381E7B768891E4DA3843D24D94CDFB5126E9B8BF21E8358EE0E0A30EF13FD6A664C0DCE3731F7FB49A4845A4FD8254687972A2D382599C9BAC4E0ED7998193078913032558134976410B89D2C171D123AC35FD977219597AA7D15C1A9A428E59194F75C721EBCBCFAE44696A499AFA74E04299F132026601638CB87AB79190D4A0986315DA8EEC6561C938996BEADF"),
		},
		X: dsaLoadInt("69C7548C21D0DFEA6B9A51C9EAD4E27C33D3B3F180316E5BCAB92C933F0E4DBC"),
	},
	subgroup: 256,
}

var dsaFixtures = []dsaFixture{
	// DSA, 1024 Bits
	// https://tools.ietf.org/html/rfc6979#appendix-A.2.1
	{
		name:    "1024/SHA-1 #1",
		key:     dsa1024,
		alg:     sha1.New,
		message: "sample",
		r:       "2E1A0C2562B2912CAAF89186FB0F42001585DA55",
		s:       "29EFB6B0AFF2D7A68EB70CA313022253B9A88DF5",
	},
	{
		name:    "1024/SHA-224 #1",
		key:     dsa1024,
		alg:     sha256.New224,
		message: "sample",
		r:       "4BC3B686AEA70145856814A6F1BB53346F02101E",
		s:       "410697B92295D994D21EDD2F4ADA85566F6F94C1",
	},
	{
		name:    "1024/SHA-256 #1",
		key:     dsa1024,
		alg:     sha256.New,
		message: "sample",
		r:       "81F2F5850BE5BC123C43F71A3033E9384611C545",
		s:       "4CDD914B65EB6C66A8AAAD27299BEE6B035F5E89",
	},
	{
		name:    "1024/SHA-384 #1",
		key:     dsa1024,
		alg:     sha512.New384,
		message: "sample",
		r:       "07F2108557EE0E3921BC1774F1CA9B410B4CE65A",
		s:       "54DF70456C86FAC10FAB47C1949AB83F2C6F7595",
	},
	{
		name:    "1024/SHA-512 #1",
		key:     dsa1024,
		alg:     sha512.New,
		message: "sample",
		r:       "16C3491F9B8C3FBBDD5E7A7B667057F0D8EE8E1B",
		s:       "02C36A127A7B89EDBB72E4FFBC71DABC7D4FC69C",
	},
	{
		name:    "1024/SHA-1 #2",
		key:     dsa1024,
		alg:     sha1.New,
		message: "test",
		r:       "42AB2052FD43E123F0607F115052A67DCD9C5C77",
		s:       "183916B0230D45B9931491D4C6B0BD2FB4AAF088",
	},
	{
		name:    "1024/SHA-224 #2",
		key:     dsa1024,
		alg:     sha256.New224,
		message: "test",
		r:       "6868E9964E36C1689F6037F91F28D5F2C30610F2",
		s:       "49CEC3ACDC83018C5BD2674ECAAD35B8CD22940F",
	},
	{
		name:    "1024/SHA-256 #2",
		key:     dsa1024,
		alg:     sha256.New,
		message: "test",
		r:       "22518C127299B0F6FDC9872B282B9E70D0790812",
		s:       "6837EC18F150D55DE95B5E29BE7AF5D01E4FE160",
	},
	{
		name:    "1024/SHA-384 #2",
		key:     dsa1024,
		alg:     sha512.New384,
		message: "test",
		r:       "854CF929B58D73C3CBFDC421E8D5430CD6DB5E66",
		s:       "91D0E0F53E22F898D158380676A871A157CDA622",
	},
	{
		name:    "1024/SHA-512 #2",
		key:     dsa1024,
		alg:     sha512.New,
		message: "test",
		r:       "8EA47E475BA8AC6F2D821DA3BD212D11A3DEB9A0",
		s:       "7C670C7AD72B6C050C109E1790008097125433E8",
	},

	// DSA, 2048 Bits
	// https://tools.ietf.org/html/rfc6979#appendix-A.2.2
	{
		name:    "2048/SHA-1 #1",
		key:     dsa2048,
		alg:     sha1.New,
		message: "sample",
		r:       "3A1B2DBD7489D6ED7E608FD036C83AF396E290DBD602408E8677DAABD6E7445A",
		s:       "D26FCBA19FA3E3058FFC02CA1596CDBB6E0D20CB37B06054F7E36DED0CDBBCCF",
	},
	{
		name:    "2048/SHA-224 #1",
		key:     dsa2048,
		alg:     sha256.New224,
		message: "sample",
		r:       "DC9F4DEADA8D8FF588E98FED0AB690FFCE858DC8C79376450EB6B76C24537E2C",
		s:       "A65A9C3BC7BABE286B195D5DA68616DA8D47FA0097F36DD19F517327DC848CEC",
	},
	{
		name:    "2048/SHA-256 #1",
		key:     dsa2048,
		alg:     sha256.New,
		message: "sample",
		r:       "EACE8BDBBE353C432A795D9EC556C6D021F7A03F42C36E9BC87E4AC7932CC809",
		s:       "7081E175455F9247B812B74583E9E94F9EA79BD640DC962533B0680793A38D53",
	},
	{
		name:    "2048/SHA-384 #1",
		key:     dsa2048,
		alg:     sha512.New384,
		message: "sample",
		r:       "B2DA945E91858834FD9BF616EBAC151EDBC4B45D27D0DD4A7F6A22739F45C00B",
		s:       "19048B63D9FD6BCA1D9BAE3664E1BCB97F7276C306130969F63F38FA8319021B",
	},
	{
		name:    "2048/SHA-512 #1",
		key:     dsa2048,
		alg:     sha512.New,
		message: "sample",
		r:       "2016ED092DC5FB669B8EFB3D1F31A91EECB199879BE0CF78F02BA062CB4C942E",
		s:       "D0C76F84B5F091E141572A639A4FB8C230807EEA7D55C8A154A224400AFF2351",
	},
	{
		name:    "2048/SHA-1 #2",
		key:     dsa2048,
		alg:     sha1.New,
		message: "test",
		r:       "C18270A93CFC6063F57A4DFA86024F700D980E4CF4E2CB65A504397273D98EA0",
		s:       "414F22E5F31A8B6D33295C7539C1C1BA3A6160D7D68D50AC0D3A5BEAC2884FAA",
	},
	{
		name:    "2048/SHA-224 #2",
		key:     dsa2048,
		alg:     sha256.New224,
		message: "test",
		r:       "272ABA31572F6CC55E30BF616B7A265312018DD325BE031BE0CC82AA17870EA3",
		s:       "E9CC286A52CCE201586722D36D1E917EB96A4EBDB47932F9576AC645B3A60806",
	},
	{
		name:    "2048/SHA-256 #2",
		key:     dsa2048,
		alg:     sha256.New,
		message: "test",
		r:       "8190012A1969F9957D56FCCAAD223186F423398D58EF5B3CEFD5A4146A4476F0",
		s:       "7452A53F7075D417B4B013B278D1BB8BBD21863F5E7B1CEE679CF2188E1AB19E",
	},
	{
		name:    "2048/SHA-384 #2",
		key:     dsa2048,
		alg:     sha512.New384,
		message: "test",
		r:       "239E66DDBE8F8C230A3D071D601B6FFBDFB5901F94D444C6AF56F732BEB954BE",
		s:       "6BD737513D5E72FE85D1C750E0F73921FE299B945AAD1C802F15C26A43D34961",
	},
	{
		name:    "2048/SHA-512 #2",
		key:     dsa2048,
		alg:     sha512.New,
		message: "test",
		r:       "89EC4BB1400ECCFF8E7D9AA515CD1DE7803F2DAFF09693EE7FD1353E90A68307",
		s:       "C9F0BDABCC0D880BB137A994CC7F3980CE91CC10FAF529FC46565B15CEA854E1",
	},
}

func TestDSASignatures(t *testing.T) {
	for _, f := range dsaFixtures {
		testDsaFixture(&f, t)
	}
}

func testDsaFixture(f *dsaFixture, t *testing.T) {
	t.Logf("Testing %s", f.name)

	h := f.alg()
	h.Write([]byte(f.message))
	digest := h.Sum(nil)

	g := f.key.subgroup / 8
	if len(digest) > g {
		digest = digest[0:g]
	}

	r, s, err := rfc6979.SignDSA(f.key.key, digest, f.alg)
	if err != nil {
		t.Error(err)
		return
	}

	expectedR := dsaLoadInt(f.r)
	expectedS := dsaLoadInt(f.s)

	if r.Cmp(expectedR) != 0 {
		t.Errorf("%s: Expected R of %X, got %X", f.name, expectedR, r)
	}

	if s.Cmp(expectedS) != 0 {
		t.Errorf("%s: Expected S of %X, got %X", f.name, expectedS, s)
	}
}

func dsaLoadInt(s string) *big.Int {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return new(big.Int).SetBytes(b)
}
