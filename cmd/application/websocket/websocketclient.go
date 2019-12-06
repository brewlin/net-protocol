package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/protocol/application/websocket"
)

func init()  {
	logging.Setup()

}
func main(){
	wscli ,_ := websocket.NewClient("http://10.0.2.15:8080/ws")
	defer wscli.Close()
	//升级 http协议为websocket
	if err := wscli.Upgrade();err != nil {
		panic(err)
	}
	//循环接受数据
	for {
		if err := wscli.Push("test");err != nil {
			break
		}
		data,_ := wscli.Recv()
		fmt.Println(data)
	}
}