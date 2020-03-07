package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/protocol/application/dns"
	"github.com/brewlin/net-protocol/protocol/header"
)

func main() {
	d := dns.NewEndpoint("www.baidu.com")
	fmt.Println("DNS lookuphost    : www.baidu.com")
	defer d.Close()

	ir,err := d.Resolve();
	if err != nil {
		fmt.Println(err)
		return
	}
	for _,v := range *ir {
		switch v.Type {
		case header.A:
			fmt.Println("A(host name)      :",v.Address)
		case header.CNAME:
			fmt.Println("CNAME (alias name):",v.Address)
		}
	}

	
}
