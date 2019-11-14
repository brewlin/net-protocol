package main

import (
	"fmt"

	"github.com/brewlin/net-protocol/protocol/link/tuntap"
)

var name = "tap"

func main() {
	//关闭网卡
	if err := tuntap.DelTap(name); err != nil {
		fmt.Println(err)
		return
	}
}
