package main

import (
	"log"

	"github.com/brewlin/net-protocol/link/rawfile"
	"github.com/brewlin/net-protocol/link/tuntap"
)

func main() {
	tapName := "tap0"
	c := &tuntap.Config{tapName, tuntap.TAP}
	fd, err := tuntap.NewNetDev(c)
	if err != nil {
		panic(err)
	}

	//启动网卡
	tuntap.SetLinkUp(tapName)
	//添加IP地址
	// tuntap.AddIP(tapName, "192.168.1.1")
	//设置路由
	tuntap.SetRoute(tapName, "192.168.1.0/24")
	buf := make([]byte, 1<<16)
	for {
		rn, err := rawfile.BlockingRead(fd, buf)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println("read %d bytes", rn)
	}
}
