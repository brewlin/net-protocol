package client

import (
	"github.com/brewlin/net-protocol/pkg/buffer"
	tcpip "github.com/brewlin/net-protocol/protocol"
)
//Write
func (c *Client) Write(buf []byte) *tcpip.Error {
	v := buffer.View(buf)
	for{
		_,ch,err := c.ep.Write(tcpip.SlicePayload(v),
			tcpip.WriteOptions{To: &c.remote})
		if err == tcpip.ErrWouldBlock {
			<-ch
			continue
		}
		return err
	}
}