package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/brewlin/net-protocol/pkg/waiter"
	"github.com/brewlin/net-protocol/protocol/link/fdbased"
	"github.com/brewlin/net-protocol/protocol/link/tuntap"
	"github.com/brewlin/net-protocol/protocol/network/arp"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/network/ipv6"
	"github.com/brewlin/net-protocol/protocol/transport/tcp"
	"github.com/brewlin/net-protocol/protocol/transport/udp"
	"github.com/brewlin/net-protocol/stack"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

// var mac = flag.String("mac", "01:01:01:01:01:01", "mac address to use in tap device")
var mac = flag.String("mac", "aa:00:01:01:01:01", "mac address to use in tap device")

func main() {
	flag.Parse()
	if len(flag.Args()) != 4 {
		log.Fatal("usage: ", os.Args[0], " < tap-device> <local-address/mask> <ip-address> <port>")
	}

	log.SetFlags(log.Lshortfile)
	tapName := flag.Arg(0)
	cidrName := flag.Arg(1)
	addrName := flag.Arg(2)
	portName := flag.Arg(3)

	log.Printf("tap: %v,addr:%v,port:%v", tapName, addrName, portName)

	//解析mac地址
	maddr, err := net.ParseMAC(*mac)
	if err != nil {
		log.Fatalf("Bad mac address:%v", *mac)
	}
	parseAddr := net.ParseIP(addrName)
	if err != nil {
		log.Fatalf("bad address:%v", addrName)
	}

	//解析ip地址，ipv4 或者ipv6
	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber
	if parseAddr.To4() != nil {
		addr = tcpip.Address(parseAddr.To4())
		proto = ipv4.ProtocolNumber
	} else if parseAddr.To16() != nil {
		addr = tcpip.Address(parseAddr.To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatalf("Unknow IP type:%v", parseAddr)
	}

	localPort, err := strconv.Atoi(portName)
	if err != nil {
		log.Fatalf("unable to convert port %v:%v", portName, err)
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
	//启动tap网卡
	tuntap.SetLinkUp(tapName)
	//设置路由
	tuntap.SetRoute(tapName, cidrName)

	//抽象网卡的文件接口 实现的接口
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

	var wq waiter.Queue
	//新建一个UDP端
	ep, e := s.NewEndpoint(udp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal(e)
	}
	//绑定本地端口
	if err := ep.Bind(tcpip.FullAddress{1, addr, uint16(localPort)}, nil); err != nil {
		log.Fatal("@main : bind failed :", err)
	}
	echo(&wq, ep)

}
func echo(wq *waiter.Queue, ep tcpip.Endpoint) {
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
		log.Printf("@main :read and write data:%s", string(v))
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
