package keys

import (
	"encoding/base64"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/crypto/hash"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestVerify(t *testing.T) {
	crypto.GenerateKey()
	k, err := NewPrivateKey()
	require.NoError(t, err)
	msg, err := base64.StdEncoding.DecodeString("JFp8qVHjRNxXWuQvjLOiCt4YYGVMA4Jxw+BaLitgaA0=")
	require.NoError(t, err)
	sig := k.Sign(msg)
	t.Log(base64.StdEncoding.EncodeToString(k.PublicKey().getBytes(true)))
	t.Log(base64.StdEncoding.EncodeToString(sig))

	t.Log(k.PublicKey().Verify(sig, hash.Sha256(msg).Bytes()))
}
