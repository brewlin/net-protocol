package main

import (
	"flag"
	"log"
	"net"
	"os"

	tcpip "github.com/brewlin/net-protocol/protocol"

	"github.com/brewlin/net-protocol/protocol/link/fdbased"
	"github.com/brewlin/net-protocol/protocol/link/tuntap"
	"github.com/brewlin/net-protocol/protocol/network/arp"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/network/ipv6"
	"github.com/brewlin/net-protocol/stack"
)

func main() {
	log.SetFlags(log.Lshortfile)
	//解析命令行参数
	flag.Parse()
	if len(flag.Args()) < 2 {
		log.Fatal("Usage: ", os.Args[0], " <tap-device> <local-address/mask")
	}

	tapName := flag.Arg(0)
	cidrName := flag.Arg(1)
	log.Printf("tap :%v,cidrName :%v", tapName, cidrName)

	parseAddr, cidr, err := net.ParseCIDR(cidrName)
	if err != nil {
		log.Fatalf("Bad cidr:%v", cidrName)
	}

	//解析地址ip地址，ipv4 或者ipv6 地址都支持
	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber
	if parseAddr.To4() != nil {
		addr = tcpip.Address(parseAddr.To4())
		proto = ipv4.ProtocolNumber
	} else if parseAddr.To16() != nil {
		addr = tcpip.Address(parseAddr.To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatalf("Unknown IP type:%v", parseAddr)
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
	tuntap.SetRoute(tapName, cidr.String())

	//获取mac地址
	mac, err := tuntap.GetHardwareAddr(tapName)
	log.Println("get mac addr:", string(mac))

	if err != nil {
		panic(err)
	}
	//抽象网卡的文件接口
	linkID := fdbased.New(&fdbased.Options{
		FD:      fd,
		MTU:     1500,
		Address: tcpip.LinkAddress(mac),
	})

	//新建相关协议
	s := stack.New([]string{ipv4.ProtocolName, arp.ProtocolName}, []string{}, stack.Options{})

	//新建抽象的网卡
	// if err := s.CreateNamedNIC(1, "vnic1", linkID); err != nil {
	if err := s.CreateNamedNIC(1, "vnic1", linkID); err != nil {
		log.Fatal(err)
	}
	//在该协议栈上添加和注册相应的网络层
	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal(err)
	}

	//在该协议栈上添加和注册ARP协议
	if err := s.AddAddress(1, arp.ProtocolNumber, arp.ProtocolAddress); err != nil {
		log.Fatal(err)
	}
	select {}

}
