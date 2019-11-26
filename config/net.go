package config

import (
	"flag"
	tcpip "github.com/brewlin/net-protocol/protocol"
	"net"
)
//mac地址
var Mac = flag.String("mac", "aa:00:01:01:01:01", "mac address to use in tap device")
//网卡名
var NicName = "tap"
//路由网段
var Cidrname = "192.168.1.0/24"
//ip网段
var Ipname = "192.168.1.1/24"
//网关配置
var DestinationNet = "default"
var GatewayNet = "192.168.1.2"

var LocalAddres = tcpip.Address(net.ParseIP("192.168.1.1").To4())