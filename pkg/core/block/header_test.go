package block

import (
	"testing"

	"github.com/DigitalLabs-web3/neo-go-evm/pkg/io"
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
