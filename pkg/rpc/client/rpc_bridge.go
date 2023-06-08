package client

import (
	"github.com/DigitalLabs-web3/neo-go-evm/pkg/rpc/request"
	"github.com/ethereum/go-ethereum/common"
)

func (c *Client) Bridge_GetMinted(id int64) (common.Hash, error) {
	var (
		params = request.NewRawParams()
		resp   = common.Hash{}
	)
	err := c.performRequest("bridge_getMinted", params, &resp)
	return resp, err
}
