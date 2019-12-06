package websocket

import "C"
import (
	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/protocol/application/http"
)

type Client struct {
	httpClient *http.Client
	con *Conn
}
//NewCient new http client
//NewClient("http://10.0.2.15:8080/")
func NewClient(url string)(*Client,error){
	cli,err := http.NewClient(url)
	if err != nil {
		panic(err)
	}
	return &Client{
		httpClient: cli,
		con:        nil,
	},nil
}
//Upgrade http 协议升级为websocket协议
//GET /path HTTP/1.1
//Host: server.example.com
//Upgrade: websocket
//Connection: Upgrade
//Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
//Origin: http://example.com
//Sec-WebSocket-Protcol: chat, superchat
//Sec-WebSocket-Version: 13
func (c *Client)Upgrade()error{
	headers := map[string]string{
		"Upgrade":"websocket",
		"Connection":"Upgrade",
		"Sec-WebSocket-Key":buffer.GetRandomString(24),
		"Sec-WebSocket-Protcol":"chat, superchat",
		"Sec-WebSocket-Version":"13",
	}
	c.httpClient.SetHeaders(headers)
	if err := c.httpClient.Push(); err != nil {
		return err
	}
	//这里进行校验 websocket回复是否合法，本次实现不做校验
	//c.httpClient.GetRequest().GetHeader("Sec-WebSocket-Accept") == ?
	c.con = newConn(c.httpClient.GetConnection())
	return nil
}
//Push push to websocket server data
func (c *Client)Push(data string) error{
	return c.con.SendData([]byte(data))
}
//Recv recvData
func (c *Client)Recv()(string,error) {
	data,err := c.con.ReadData()
	return string(data),err
}
func (c *Client)Close()  {
	c.con.Close()
}