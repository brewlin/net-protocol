package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/brewlin/net-protocol/pkg/logging"

	"github.com/brewlin/net-protocol/protocol/link/fdbased"
	"github.com/brewlin/net-protocol/protocol/link/tuntap"
	"github.com/brewlin/net-protocol/protocol/transport/udp"

	"github.com/brewlin/net-protocol/protocol/network/ipv6"

	"github.com/brewlin/net-protocol/pkg/waiter"

	"github.com/brewlin/net-protocol/protocol/network/arp"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/transport/tcp"
	"github.com/brewlin/net-protocol/stack"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

var mac = flag.String("mac", "aa:00:01:01:01:01", "mac address to use in tap device")

func init() {
	logging.Setup()
}

func main() {
	flag.Parse()
	if len(flag.Args()) != 4 {
		log.Fatal("usage:", os.Args[0], "<tap-device> <local-address/mask> <ipv4-address> <port>")
	}
	tapName := flag.Arg(0)
	cidrName := flag.Arg(1)
	addrName := flag.Arg(2)
	portName := flag.Arg(3)
	log.Printf("tap :%v addr :%v port :%v", tapName, addrName, portName)

	//解析mac地址
	maddr, err := net.ParseMAC(*mac)
	if err != nil {
		log.Fatal(*mac)
	}
	parseAddr := net.ParseIP(addrName)
	if err != nil {
		log.Fatal("BAD ADDRESS", addrName)
	}
	//解析IP地址，ipv4,或者ipv6
	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber
	if parseAddr.To4() != nil {
		addr = tcpip.Address(net.ParseIP(addrName).To4())
		proto = ipv4.ProtocolNumber
	} else if parseAddr.To16() != nil {
		addr = tcpip.Address(net.ParseIP(addrName).To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatal("unkonw iptype")
	}
	localPort, err := strconv.Atoi(portName)
	if err != nil {
		log.Fatalf("unable to convert port")
	}

	//虚拟网卡配置
	conf := &tuntap.Config{
		Name: tapName,
		Mode: tuntap.TAP,
	}

	var fd int
	//新建虚拟网卡
	fd, err = tuntap.NewNetDev(conf)
	if err != nil {
		log.Fatal(err)
	}
	//启动网卡
	tuntap.SetLinkUp(tapName)
	//设置路由
	tuntap.SetRoute(tapName, cidrName)

	//抽象网卡层接口
	linkID := fdbased.New(&fdbased.Options{
		FD:                 fd,
		MTU:                1500,
		Address:            tcpip.LinkAddress(maddr),
		ResolutionRequired: true,
	})
	//新建相关协议的协议栈
	s := stack.New([]string{ipv4.ProtocolName, arp.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName}, stack.Options{})
	//新建抽象网卡
	if err := s.CreateNamedNIC(1, "vnic1", linkID); err != nil {
		log.Fatal(err)
	}

	//在该协议栈上添加和注册相关的网络层协议
	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal(err)
	}

	//在该协议栈上添加和注册arp协议
	if err := s.AddAddress(1, arp.ProtocolNumber, arp.ProtocolAddress); err != nil {
		log.Fatal(err)
	}
	//添加默认路由
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: tcpip.Address(strings.Repeat("\x00", len(addr))),
			Mask:        tcpip.AddressMask(strings.Repeat("\x00", len(addr))),
			Gateway:     "",
			NIC:         1,
		},
	})

	tcpServer(s, addr, localPort)

}

func tcpServer(s *stack.Stack, addr tcpip.Address, port int) {
	var wq waiter.Queue
	//新建一个tcp端
	ep, e := s.NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if e != nil {
		log.Fatal(e)
	}
	//绑定本地端口
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(port)}, nil); err != nil {
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
