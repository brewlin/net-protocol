package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/config"
	"github.com/brewlin/net-protocol/internal/endpoint"
	"github.com/brewlin/net-protocol/pkg/logging"
	"log"

	"github.com/brewlin/net-protocol/pkg/waiter"

	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/transport/tcp"
	"github.com/brewlin/net-protocol/stack"

	tcpip "github.com/brewlin/net-protocol/protocol"
)


func init() {
	logging.Setup()
}

func main() {
	s := endpoint.NewEndpoint()
	tcpServer(s)

}

func tcpServer(s *stack.Stack) {
	var wq waiter.Queue
	//新建一个tcp端
	ep, e := s.NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if e != nil {
		log.Fatal(e)
	}
	//绑定本地端口
	if err := ep.Bind(tcpip.FullAddress{0, "", config.LocalPort}, nil); err != nil {
		log.Fatal("@main :Bind failed: ", err)
	}
	//监听tcp
	if err := ep.Listen(10); err != nil {
		log.Fatal("@main :Listen failed: ", err)
	}
	//等待连接 出现
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		n, q, err := ep.Accept()
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				fmt.Println("@main server:", " now waiting to new client connection ...")
				<-notifyCh
				continue
			}
			fmt.Println("@main server: Accept() failed: ", err)
			panic(err)
		}
		addr, _ := n.GetRemoteAddress()
		fmt.Println("@main server: new client connection : ", addr)

		go dispatch(n, q, addr)
	}
}
func dispatch(e tcpip.Endpoint, wq *waiter.Queue, addr tcpip.FullAddress) {

	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)
	for {
		v, c, err := e.Read(&addr)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				fmt.Println("@main dispatch: waiting new event trigger ...")
				<-notifyCh
				continue
			}
			fmt.Println("@main dispatch:tcp read  got error", err)
			break
		}
		fmt.Println("@main dispatch: recv ", v, c)
		a, b, er := e.Write(tcpip.SlicePayload(v), tcpip.WriteOptions{To: &addr})
		fmt.Println("@main dispatch: write to client res: ", a, b, er)
	}
	e.Close()
}
