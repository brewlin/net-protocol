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
	c := NewCon(fd)
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
//SetHeaders
func (c *Client)SetHeaders(headers map[string]string){
	for k,v := range headers {
		c.req.headers.ptr[k] = v
	}
}
//SetData
func (c *Client)SetData(buf string) {
	c.req.body = buf
}
//GetResult
func (c *Client)GetResult()(string,error)  {
	if err := c.Push();err != nil {
		return "",err
	}
	return c.con.request.GetBody(),nil
}
//GetBody
func (c *Client)Push()(err error) {
	buf := c.req.send()
	if err = c.client.Write([]byte(buf)); err != nil {
		return
	}
	recvbuf,_ := c.client.Read()
	c.con.recv_buf = string(recvbuf)
	c.con.request.parse(c.con)
	return
}
//GetConnection
func (c *Client)GetConnection()*Connection{
	return c.con
}
func (c *Client)GetRequest()*Request{
	return c.con.request
}