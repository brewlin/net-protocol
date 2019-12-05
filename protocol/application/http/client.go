package http

import (
	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/protocol/transport/tcp/client"
)

type Client struct {
	con *Connection
	client *client.Client
	req *Request
}
//NewCient new http client
//NewClient("http://10.0.2.15:8080/")
func NewClient(url string)(*Client,error){
	ip,port,path,err := buffer.ParseUrl(url)
	if err != nil {
		return nil,err
	}
	fd := client.NewClient(ip,port)
	if err := fd.Connect(); err != nil {
		return nil,err
	}
	c := newCon(fd)
	req := newRequest()
	req.init(path,ip,port)
	return &Client{
		con:c,
		client:fd,
		req:req,
	},nil
}
//SetMethod
func (c *Client)SetMethod(method string) {
	c.req.method_raw = method
}
//SetData
func (c *Client)SetData(buf string) {
	c.req.body = buf
}
//GetBody
func (c *Client)GetBody()(body string,err error) {
	buf := c.req.send()
	if err = c.client.Write([]byte(buf)); err != nil {
		return
	}
	recvbuf,_ := c.client.Read()
	c.con.recv_buf = string(recvbuf)
	c.con.request.parse(c.con)
	body = c.con.request.GetBody()
	c.client.Close()
	return
}
func (c *Client)GetRequest()*Request{
	return c.con.request
}