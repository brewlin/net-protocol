package client

import (
	"github.com/brewlin/net-protocol/pkg/waiter"
	tcpip "github.com/brewlin/net-protocol/protocol"
)

//GetAddr
func (c *Client) GetAddr() tcpip.Address {
	return c.addr
}
//GetRemoteAddr
func (c *Client) GetRemoteAddr() *tcpip.FullAddress {
	return &c.remote
}
//GetQueue 获取接收时间队列
func (c *Client)GetQueue()*waiter.Queue  {
	return &c.queue
}
//GetNotify 获取事件chan
func (c *Client)GetNotify()chan struct{}{
	return c.notifyC
}
