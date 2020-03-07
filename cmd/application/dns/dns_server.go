package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/config"
	"github.com/brewlin/net-protocol/internal/endpoint"
	"github.com/brewlin/net-protocol/pkg/buffer"
	_ "github.com/brewlin/net-protocol/pkg/logging"
	_ "github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/pkg/waiter"
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/transport/udp"
	"github.com/brewlin/net-protocol/protocol/transport/udp/client"
	"github.com/brewlin/net-protocol/stack"
	"log"
	"strconv"
	"strings"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

//当前demo作为一个dns代理，接受dns请求并转发后，解析响应做一些操作
func main() {
	s := endpoint.NewEndpoint()

	udploop(s)

}
func udploop(s *stack.Stack) {
	var wq waiter.Queue
	//新建一个UDP端
	ep, err := s.NewEndpoint(udp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if err != nil {
		log.Fatal(err)
	}
	//绑定本地端口  53是dns默认端口
	if err := ep.Bind(tcpip.FullAddress{1, config.LocalAddres, 53}, nil); err != nil {
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
			fmt.Println(err)
			return
		}
		//接收到代理请求
		h := header.DNS(v)
		fmt.Println("@main :接收到代理域名:", string(h[header.DOMAIN:header.DOMAIN+h.GetDomainLen()-1]))
		go handle_proxy(v,ep,saddr)
	}
}
//转发代理请求，并解析响应数据
func handle_proxy(v buffer.View,ep tcpip.Endpoint,saddr tcpip.FullAddress){
	cli := client.NewClient("8.8.8.8",53)
	cli.Connect()
	cli.Write(v)
	defer cli.Close()

	rsp,err := cli.Read()
	if err != nil {
		fmt.Println(err)
		return
	}
	//返回给客户端
	_, _, err = ep.Write(tcpip.SlicePayload(rsp), tcpip.WriteOptions{To: &saddr})
	if err != nil {
		fmt.Println(err)
	}
	p := header.DNS(rsp)
	answer := p.GetAnswer()

	for i := 0; i < len(*answer) ; i++ {
		switch (*answer)[i].Type {
		case header.A:
			fmt.Println("dns 目标IP（A):",parseAName((*answer)[i].RData))
		case header.CNAME:
			fmt.Println("dns 目标IP（alias):",parseCName((*answer)[i].RData))
		}
	}
}
func parseAName(rd []byte) string {
	res := []string{}
	for _,v := range rd {
		res = append(res,strconv.Itoa(int(v)))
	}
	return strings.Join(res,".")
}

func parseCName(rd []byte) (res string) {
	for{
		l := int(rd[0])
		if l >= len(rd){
			res += ".com"
			return
		}
		rd = rd[1:]
		res += string(rd[0:l])
		rd = rd[l:]
		if len(rd) == 0 {
			return
		}
	}
}