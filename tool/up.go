package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/config"
	"os/exec"

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
	//if err := tuntap.AddIP(config.NicName, config.Ipname); err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//添加路由
	if err := tuntap.SetRoute(config.NicName, config.Cidrname); err != nil {
		fmt.Println(err)
		return
	}
	////添加网关
	//if err := tuntap.AddGateWay(config.DestinationNet, config.GatewayNet, config.NicName); err != nil {
	//	fmt.Println(err)
	//	return
	//}
}
func IpForwardAndNat()(err error){
	out, cmdErr := exec.Command("iptables", "-F").CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}

	out, cmdErr = exec.Command("iptables", "-P","INPUT ","ACCEPT").CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	out, cmdErr = exec.Command("iptables", "-P","FORWARD  ","ACCEPT").CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	out, cmdErr = exec.Command("iptables", "-t","nat","-A","POSTROUTING","-s",config.Cidrname,"-o",config.HardwardName,"-j","MASQUERADE").CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	return
}
