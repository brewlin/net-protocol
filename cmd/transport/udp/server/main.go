package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/config"
	"github.com/brewlin/net-protocol/internal/endpoint"
	"github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/pkg/waiter"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/transport/tcp"
	"github.com/brewlin/net-protocol/protocol/transport/udp"
	"github.com/brewlin/net-protocol/stack"
	"log"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

func init() {
	logging.Setup()
}
func main() {
	s := endpoint.NewEndpoint()

	echo(s)

}
func echo(s *stack.Stack) {
	var wq waiter.Queue
	//新建一个UDP端
	ep, err := s.NewEndpoint(udp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if err != nil {
		log.Fatal(err)
	}
	//绑定本地端口
	if err := ep.Bind(tcpip.FullAddress{1, config.LocalAddres, config.LocalPort}, nil); err != nil {
		log.Fatal("@main : bind failed :", err)
	}
	defer ep.Close()
	//创建队列 通知 channel
	waitEntry, notifych := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	var saddr tcpip.FullAddress

	for {
		v, _, err := ep.Read(&saddr)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifych
				continue
			}
			return
		}
		fmt.Printf("@main :read and write data:%s", string(v))
		_, _, err = ep.Write(tcpip.SlicePayload(v), tcpip.WriteOptions{To: &saddr})
		if err != nil {
			log.Fatal(err)
		}
	}
}
func tcpListen(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int) tcpip.Endpoint {
	var wq waiter.Queue
	//新建一个tcp端
	ep, err := s.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal(err)
	}
	//绑定ip和端口，这里的ip地址为空，表示绑定任何ip
	//此时就会调用端口管理器
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort)}, nil); err != nil {
		log.Fatal("@main : Bind failed: ", err)
	}
	//开始监听
	if err := ep.Listen(10); err != nil {
		log.Fatal("@main : Listen failed : ", err)
	}
	return ep
}

func udpListen(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int) tcpip.Endpoint {
	var wq waiter.Queue
	//新建一个udp端
	ep, err := s.NewEndpoint(udp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal(err)
	}
	//绑定ip和端口，这里的ip地址为空，表示绑定任何ip
	//此时就会调用端口管理器
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort)}, nil); err != nil {
		log.Fatal("@main :Bind failed: ", err)
	}
	//注意udp是无连接的，它不需要listen
	return ep
}
