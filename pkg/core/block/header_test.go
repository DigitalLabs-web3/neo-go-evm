package block

import (
	"encoding/base64"
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
	"github.com/stretchr/testify/require"
)

func TestHeaderEncode(t *testing.T) {
	header := Header{}
	t.Log(header.Hash())
	w := io.NewBufBinWriter()
	header.EncodeBinary(w.BinWriter)
	b := w.Bytes()
	h := new(Header)
	r := io.NewBinReaderFromBuf(b)
	h.DecodeBinary(r)
	t.Log(h.Hash())
}

func TestVerify(t *testing.T) {
	b, err := base64.StdEncoding.DecodeString("AAAAAK2I4xG0Pj0ENGNioqq+mroYg4c5k36fnPPudURtC067AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADPX61kAAAAAEESIoHCsbzyAwAAAABThnW9co9XHX2VUmOzGYgMUIEnn0IBQEyeOQTb3lJ2BDSRSyZWmCVkuPbgC+qgJGTGs09nTGQTI8/oDJ1XOkJb3DTCEJ3oBo7ojZRCGBT0GoNMdKtONF8jAQEDvqlYKfd35EWjOLlQT5OPMejGrfRikvaspMBIYoVlHjk=")
	require.NoError(t, err)
	header := new(Header)
	require.NoError(t, io.FromByteArray(header, b))
	t.Log(header.Witness.VerifyHashable(4, header))
}
