package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/config"

	"github.com/brewlin/net-protocol/protocol/link/tuntap"
)

func main() {
	//关闭网卡
	if err := tuntap.DelTap(config.NicName); err != nil {
		fmt.Println(err)
		return
	}
}
