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
	//添加路由
	if err := tuntap.SetRoute(config.NicName, config.Cidrname); err != nil {
		fmt.Println(err)
		return
	}
	//开启防火墙规则 nat数据包转发
	if err := IpForwardAndNat();err != nil {
		fmt.Println(err)
		tuntap.DelTap(config.NicName)
		return
	}
}
func IpForwardAndNat()(err error){
	//清楚本地物联网看的数据包规则， 模拟防火墙
	//out, cmdErr := exec.Command("iptables", "-F").CombinedOutput()
	//if cmdErr != nil {
	//	err = fmt.Errorf("iptables -F %v:%v", cmdErr, string(out))
	//	return
	//}

	out, cmdErr := exec.Command("iptables", "-P","INPUT","ACCEPT").CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("iptables -P INPUT ACCEPT %v:%v", cmdErr, string(out))
		return
	}
	out, cmdErr = exec.Command("iptables", "-P","FORWARD","ACCEPT").CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("iptables -P FORWARD ACCEPT %v:%v", cmdErr, string(out))
		return
	}
	out, cmdErr = exec.Command("iptables", "-t","nat","-A","POSTROUTING","-s",config.Cidrname,"-o",config.HardwardName,"-j","MASQUERADE").CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("iptables nat %v:%v", cmdErr, string(out))
		return
	}
	return
}
