package client

import (
	"errors"
	"log"
	"net"

	"github.com/brewlin/net-protocol/protocol/transport/udp"

	"github.com/brewlin/net-protocol/pkg/waiter"

	"sync"

	"github.com/brewlin/net-protocol/pkg/buffer"
	tcpip "github.com/brewlin/net-protocol/protocol"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/stack"
	"github.com/brewlin/net-protocol/stack/stackinit"
)

//Client client struct
type Client struct {
	s    *stack.Stack
	ep   tcpip.Endpoint
	addr tcpip.Address
	port int

	//接受队列缓存区
	buf   buffer.View
	bufmu sync.RWMutex

	notifyC   chan struct{}
	waitEntry waiter.Entry

	remote tcpip.FullAddress
	queue  waiter.Queue
}

//NewClient get new udp client
func NewClient(addrName string, port int) *Client {
	addr := tcpip.Address(net.ParseIP(addrName).To4())
	return &Client{
		addr: tcpip.Address(addr),
		port: port,
	}
}

//Set set options
func (c *Client) Set(s *stack.Stack) {
	c.s = s
}

//Connect connect
func (c *Client) Connect() error {
	c.s = stack.Pstack
	if c.s == nil {
		log.Println("stack is nil")
		return errors.New("stack is nil")
	}
	return c.connect(c.s)
}

func (c *Client) connect(s *stack.Stack) error {
	//添加路由
	stackinit.AddRoute(c.addr)
	c.remote = tcpip.FullAddress{
		Addr: c.addr,
		Port: uint16(c.port),
	}
	var wq waiter.Queue
	//新建一个udp端
	ep, err := s.NewEndpoint(udp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if err != nil {
		log.Println(err)
		return err
	}
	c.ep = ep
	c.queue = wq

	c.waitEntry, c.notifyC = waiter.NewChannelEntry(nil)
	wq.EventRegister(&c.waitEntry, waiter.EventOut|waiter.EventIn)
	c.ep = ep
	terr := c.ep.Connect(c.remote)
	if terr == tcpip.ErrConnectStarted {
		log.Println("@传输层 udp/client : Connect is pending...")
		<-c.notifyC
		terr = ep.GetSockOpt(tcpip.ErrorOption{})
	}
	if terr != nil {
		log.Println("@传输层 udp/client : Unable to connect: ", terr)
		return terr
	}
	log.Println("@传输层 udp/client:Connected")
	return nil
}
func (c *Client) Close() {
	c.queue.EventUnregister(&c.waitEntry)
	c.ep.Close()
	log.Println("@传输层 udp/client :tcp disconnected")
}
