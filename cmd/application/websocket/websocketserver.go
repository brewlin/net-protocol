package main

import (
	"fmt"
	"github.com/brewlin/net-protocol/config"
	"log"

	"github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/protocol/application/http"
	"github.com/brewlin/net-protocol/protocol/application/websocket"
)

func init() {
	logging.Setup()
}
func main() {
	serv := http.NewHTTP(config.NicName, "192.168.1.0/24", "192.168.1.1", "9502")
	serv.HandleFunc("/ws", echo)

	serv.HandleFunc("/", func(request *http.Request, response *http.Response) {
		response.End("hello")
	})
	fmt.Println("@main: server is start ip:192.168.1.1 port:9502 ")
	serv.ListenAndServ()
}

//websocket处理器
func echo(r *http.Request, w *http.Response) {
	fmt.Println("got http request ; start to  upgrade websocket protocol....")
	//协议升级 c *websocket.Conn
	c, err := websocket.Upgrade(r, w)
	if err != nil {
		//升级协议失败，直接return 交由http处理响应
		fmt.Println("Upgrade error:", err)
		return
	}
	defer c.Close()
	//循环处理数据，接受数据，然后返回
	for {
		message, err := c.ReadData()
		if err != nil {
			log.Println("read:", err)
			break
		}
		fmt.Println("recv client msg:", string(message))
		// c.SendData(message )
		c.SendData([]byte("hello"))
	}
}
