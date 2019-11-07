package stackinit

import (
	"flag"
	"log"
	"net"
	"strings"

	"github.com/brewlin/net-protocol/protocol/link/fdbased"
	"github.com/brewlin/net-protocol/protocol/link/tuntap"
	"github.com/brewlin/net-protocol/protocol/transport/udp"

	"github.com/brewlin/net-protocol/protocol/network/arp"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/transport/tcp"
	"github.com/brewlin/net-protocol/stack"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

var mac = flag.String("mac", "aa:00:01:01:01:01", "mac address to use in tap device")
var tapName = "tap1"
var cidrName = "192.168.1.0/24"

//SetRoute 设置该路由信息
func AddRoute(addr tcpip.Address) {
	var proto = ipv4.ProtocolNumber

	//在该协议栈上添加和注册相关的网络层协议
	if err := stack.Pstack.AddAddress(1, proto, addr); err != nil {
		log.Fatal(err)
	}
	//添加默认路由
	// stack.Pstack.AddRouteTable(tcpip.Route{

	// 	Destination: addr,
	// 	Mask:        tcpip.AddressMask(addr),
	// 	Gateway:     "",
	// 	NIC:         1,
	// })
	stack.Pstack.AddRouteTable(tcpip.Route{

		Destination: tcpip.Address(strings.Repeat("\x00", len(addr))),
		Mask:        tcpip.AddressMask(strings.Repeat("\x00", len(addr))),
		Gateway:     "",
		NIC:         1,
	})
}
func init() {
	//如果已经存在 p 指向的stack 则不需要在初始化
	if stack.Pstack != nil {
		return
	}
	log.Printf("tap :%v", tapName)

	//解析mac地址
	maddr, err := net.ParseMAC(*mac)
	if err != nil {
		log.Fatal(*mac)
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
	//在该协议栈上添加和注册arp协议
	if err := s.AddAddress(1, arp.ProtocolNumber, arp.ProtocolAddress); err != nil {
		log.Fatal(err)
	}
	// stack.Pstack.AddRouteTable(tcpip.Route{

	// 	Destination: tcpip.Address(strings.Repeat("\x00", len("0"))),
	// 	Mask:        tcpip.AddressMask(strings.Repeat("\x00", len(addr))),
	// 	Gateway:     "",
	// 	NIC:         1,
	// })

}
