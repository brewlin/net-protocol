package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/config"

	"github.com/brewlin/net-protocol/protocol/link/tuntap"
)

func main() {
	//创建网卡
	if err := tuntap.CreateTap(config.NicName); err != nil {
		fmt.Println(err)
		return
	}
	//启动网卡
	if err := tuntap.SetLinkUp(config.NicName); err != nil {
		fmt.Println(err)
		return
	}
	// //增加ip地址
	// if err := tuntap.AddIP(name, ipname); err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	//添加路由
	if err := tuntap.SetRoute(config.NicName, config.Cidrname); err != nil {
		fmt.Println(err)
		return
	}
	//添加网关
	if err := tuntap.AddGateWay(config.DestinationNet, config.GatewayNet, config.NicName); err != nil {
		fmt.Println(err)
		return
	}
}
