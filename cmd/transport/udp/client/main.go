package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/protocol/transport/udp/client"
	_ "github.com/brewlin/net-protocol/pkg/logging"
)

func main()  {
	//con := client.NewClient("10.0.2.15", 9000)
	con := client.NewClient("115.159.199.89", 1212)
	if err := con.Connect(); err != nil {
		fmt.Println(err)
	}
	con.Write([]byte("send msg"))
	res, _ := con.Read()
	// var p [8]byte
	// res, _ := con.Readn(p[:1])
	// fmt.Println(p)
	fmt.Println("res")
	fmt.Println(string(res))

}
