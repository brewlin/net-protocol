package http

import (
	"errors"
	"sync"

	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/pkg/waiter"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

type ServerSocket struct {
	e    tcpip.Endpoint
	addr tcpip.FullAddress

	waitEntry waiter.Entry
	notifyC   chan struct{}
	queue     *waiter.Queue

	//接受队列缓存区
	buf   buffer.View
	bufmu sync.RWMutex
}

func NewServerSocket(e tcpip.Endpoint, q *waiter.Queue) *ServerSocket {
	s := &ServerSocket{
		e: e,
	}
	s.waitEntry, s.notifyC = waiter.NewChannelEntry(nil)
	q.EventRegister(&s.waitEntry, waiter.EventIn)
	s.addr, _ = e.GetRemoteAddress()
	s.queue = q
	return s
}

//Write write
func (s *ServerSocket) Write(buf []byte) error {
	v := buffer.View(buf)
	s.e.Write(tcpip.SlicePayload(v),
		tcpip.WriteOptions{To: &s.addr})
	return nil
}

//Read data
func (s *ServerSocket) Read() ([]byte, error) {
	<-s.notifyC
	var buf []byte
	var err error
	for {
		v, _, e := s.e.Read(&s.addr)
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
	s.bufmu.Lock()
	defer s.bufmu.Unlock()

	//获取足够长度的字节
	if len(p) > len(s.buf) {

		for {
			if len(p) <= len(s.buf) {
				break
			}
			buf, _, err := s.e.Read(s.GetRemoteAddr())
			if err != nil {
				if err == tcpip.ErrWouldBlock {
					//阻塞等待数据
					<-s.GetNotify()
					continue
				}
				return 0, err
			}
			s.buf = append(s.buf, buf...)
		}
	}
	if len(p) > len(s.buf) {
		return 0, errors.New("package len is smaller than p need")
	}

	n := copy(p, s.buf)
	s.buf = s.buf[len(p):]
	return n, nil
}
//GetAddr 获取客户端ip地址
func (s *ServerSocket)GetAddr()tcpip.Address{
	return ""
}
//GetRemoteAddr 获取远程客户端ip地址
func (s *ServerSocket)GetRemoteAddr()*tcpip.FullAddress{
	return &s.addr
}
//GetQueue 获取接收时间队列
func (s *ServerSocket)GetQueue()*waiter.Queue {
	return s.queue
}
//GetNotify
func (s *ServerSocket)GetNotify()chan struct{} {
	return s.notifyC
}
//关闭连接
func (s *ServerSocket) Close() {
	//注销接受队列
	s.queue.EventUnregister(&s.waitEntry)
	s.e.Close()

}
