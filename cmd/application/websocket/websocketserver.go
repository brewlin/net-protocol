package main

import (
	"log"

	"github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/protocol/application/http"
	"github.com/brewlin/net-protocol/protocol/application/websocket"
)

func init() {
	logging.Setup()
}
func main() {
	serv := http.NewHTTP("tap1", "192.168.1.0/24", "192.168.1.1", "9502")
	serv.HandleFunc("/websocket", echo)

	serv.HandleFunc("/", func(request *http.Request, response *http.Response) {
		response.End("hello")
	})
	serv.ListenAndServ()
}

//websocket处理器
func echo(r *http.Request, w *http.Response) {
	//协议升级
	c, err := websocket.Upgrade(r, w)
	if err != nil {
		log.Print("Upgrade error:", err)
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
		log.Printf("recv:%s", message)
		c.SendData(message)
	}
}
