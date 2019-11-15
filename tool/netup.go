package main

import (
	"fmt"

	"github.com/brewlin/net-protocol/protocol/link/tuntap"
)

var name = "tap"
var cidrname = "192.168.1.0/24"
var ipname = "192.168.1.1/24"
var destinationNet = "default"
var gatewayNet = "192.168.1.2"

func main() {
	//创建网卡
	if err := tuntap.CreateTap(name); err != nil {
		fmt.Println(err)
		return
	}
	//启动网卡
	if err := tuntap.SetLinkUp(name); err != nil {
		fmt.Println(err)
		return
	}
	// //增加ip地址
	// if err := tuntap.AddIP(name, ipname); err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	//添加路由
	if err := tuntap.SetRoute(name, cidrname); err != nil {
		fmt.Println(err)
		return
	}
	//添加网关
	if err := tuntap.AddGateWay(destinationNet, gatewayNet, name); err != nil {
		fmt.Println(err)
		return
	}
}
