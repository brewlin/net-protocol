package main

import (
	"fmt"

	_ "github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/protocol/transport/udp/client"
)

func main() {
	con := client.NewClient("10.0.2.15", 9000)
	defer con.Close()

	if err := con.Connect(); err != nil {
		fmt.Println(err)
	}
	con.Write([]byte("send msg"))
	res, err := con.Read()
	if err != nil {
		fmt.Println(err)
		con.Close()
		return
	}
	// var p [8]byte
	// res, _ := con.Readn(p[:1])
	fmt.Println(string(res))

}
