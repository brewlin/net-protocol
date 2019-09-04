package main

import (
	"flag"
	"log"
	"os"

	"github.com/brewlin/net-protocol/protocol/link/rawfile"
	"github.com/brewlin/net-protocol/protocol/link/tuntap"
)

func main() {
	flag.Parse()
	log.SetFlags(log.Lshortfile)
	if len(flag.Args()) < 2 {
		log.Fatal("Usage: ", os.Args[0], " <tap-device> <local-address/mask")
	}
	tapName := flag.Arg(0)
	route := flag.Arg(1)

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
	log.Println(tapName, route)
	tuntap.SetRoute(tapName, route)
	buf := make([]byte, 1<<16)
	for {
		_, err := rawfile.BlockingRead(fd, buf)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println("@网卡 :recv ", buf)
	}
}
