package endpoint

import (
	"github.com/brewlin/net-protocol/config"
	tcpip "github.com/brewlin/net-protocol/protocol"
	"github.com/brewlin/net-protocol/protocol/link/fdbased"
	"github.com/brewlin/net-protocol/protocol/link/tuntap"
	"github.com/brewlin/net-protocol/protocol/network/arp"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/transport/tcp"
	"github.com/brewlin/net-protocol/protocol/transport/udp"
	"github.com/brewlin/net-protocol/stack"
	"github.com/brewlin/net-protocol/stack/stackinit"
	"log"
	"net"
	_ "github.com/brewlin/net-protocol/stack/stackinit"
	"strings"
)
//NewEndpoint 新建一个网卡端，默认注册 udp tcp arp ipv4等协议
func NewEndpoint()*stack.Stack{
	//如果已经存在 p 指向的stack 则不需要在初始化
	if stack.Pstack != nil {
		stackinit.AddRoute(config.LocalAddres)
		return stack.Pstack
	}
	log.Printf("tap :%v", config.NicName)

	//解析mac地址
	maddr, err := net.ParseMAC(*config.Mac)
	if err != nil {
		log.Fatal(*config.Mac)
	}

	//虚拟网卡配置
	conf := &tuntap.Config{
		Name: config.NicName,
		Mode: tuntap.TAP,
	}
	var fd int
	//新建虚拟网卡
	fd, err = tuntap.NewNetDev(conf)
	if err != nil {
		log.Fatal(err)
	}
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
	var proto = ipv4.ProtocolNumber
	//在该协议栈上添加和注册相关的网络层协议 也就是注册本地地址
	if err := s.AddAddress(1, proto, config.LocalAddres); err != nil {
		log.Fatal(err)
	}
	//在该协议栈上添加和注册arp协议
	if err := s.AddAddress(1, arp.ProtocolNumber, arp.ProtocolAddress); err != nil {
		log.Fatal(err)
	}
	//添加默认路由
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: tcpip.Address(strings.Repeat("\x00", len(config.LocalAddres))),
			Mask:        tcpip.AddressMask(strings.Repeat("\x00", len(config.LocalAddres))),
			Gateway:     "",
			NIC:         1,
		},
	})
	return s

}