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
//localip
var LocalAddres = tcpip.Address(net.ParseIP("192.168.1.1").To4())
//物理网卡ip 作为连通外网的网关使用,不填默认自动获取
var HardwardIp = "10.0.2.15"
//物理网卡名
var HardwardName = "enp0s3"