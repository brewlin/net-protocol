package main

import (
	"github.com/brewlin/net-protocol/pkg/buffer"
	"fmt"
)
func main(){
	ip,port,url,err := buffer.ParseUrl("http://10.0.2.15/test")
	fmt.Println(ip,port,url,err)
	// buffer.ParseUrl("http://10.0.2.15/test")
}