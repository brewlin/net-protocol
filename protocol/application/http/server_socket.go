package http

import (
	"flag"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/brewlin/net-protocol/protocol/link/fdbased"
	"github.com/brewlin/net-protocol/protocol/link/tuntap"
	"github.com/brewlin/net-protocol/protocol/transport/udp"

	"github.com/brewlin/net-protocol/protocol/network/ipv6"
	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/pkg/waiter"

	"github.com/brewlin/net-protocol/protocol/network/arp"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/transport/tcp"
	"github.com/brewlin/net-protocol/stack"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

type ServerSocket struct {
	e tcpip.Endpoint
	addr tcpip.FullAddress

	waitEntry waiter.Entry
	notifyC   chan struct{}
	queue *waiter.Queue
}
func NewServerSocket(e tcpip.Endpoint, q *waiter.Queue)*ServerSocket{
	s := &ServerSocket{
		e:e,
	}
	s.waitEntry, s.notifyC = waiter.NewChannelEntry(nil)
	q.EventRegister(&con.waitEntry, waiter.EventIn)
	s.addr, _ = e.GetRemoteAddress()
	s.queue = q
	return s
}
//Write write
func (s *ServerSocket) Write(buf []byte) error {
	v := buffer.View(buf)
	c.e.Write(tcpip.SlicePayload(v),
		tcpip.WriteOptions{To: s.addr})
	return nil
}
//Read data
func (s *ServerSocket) Read() ([]byte, error) {
	var buf []byte
	var err error
	for {
		v, _, e := s.e.Read(s.addr)
		if e != nil {
			err = e
			break
		}
		buf = append(buf, v...)
	}
	if buf == nil {
		return nil, err
	}
	return buf, nil

}

//Readn  读取固定字节的数据
func (s *ServerSocket) Readn(p []byte) (int, error) {
	//获取足够长度的字节
	if len(p) > len(c.buf) {

		for {
			if len(p) <= len(c.buf) {
				break
			}
			buf, _, err := s.e.Read(s.addr)
			if err != nil {
				if err == tcpip.ErrWouldBlock {
					//阻塞等待数据
					<-c.notifyC
					continue
				}
				return 0, err
			}
			c.buf = append(c.buf, buf...)
		}
	}
	if len(p) > len(c.buf) {
		return 0, errors.New("package len is smaller than p need")
	}

	n := copy(p, c.buf)
	c.buf = c.buf[len(p):]
	return n, nil
}

//关闭连接
func (s *ServerSocket) Close() {
	//注销接受队列
	s.queue.EventUnregister(&s.waitEntry)
	s.e.Close()

}
