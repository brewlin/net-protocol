package http

import (
	"github.com/brewlin/net-protocol/protocol/transport/tcp/client"
	"github.com/brewlin/net-protocol/pkg/buffer"
)

type Client struct {
	con *Connection
	client *client.Client
	path string
}
//NewCient new http client
//NewClient("http://10.0.2.15:8080/")
func NewClient(url string)*Client{
	ip,port,path,err := buffer.ParseUrl(url)
	if err != nil {
		return err
	}
	fd := client.NewClient(ip,port)
	c := newCon(fd)
	return &Client{
		con:c,
		client:fd,
		path:path,

	}
}

//Get get data
func (c *Client)Get(buf string){
	c.con.recv_buf = c.client.Read()
	c.con.request.parse(c.con)
	return c.con.request.GetBody()
}
//Post get data
func (c *Client)Post(buf string){
	c.con.recv_buf = c.client.Read()
	c.con.request.parse(c.con)
	return c.con.request.GetBody()
}
//GetRequest
func (c *Client)GetRequest()*Request{
	return c.con.request
}