package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/protocol/application/dns"
)

func main() {
	dns := dns.NewEndpoint("www.baidu.com")
	ir,err := dns.Resolve();
	fmt.Println(err)
	fmt.Println(string(ir))

	
}
