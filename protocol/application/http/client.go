package http

import (
	"github.com/brewlin/net-protocol/protocol/transport/tcp/client"
	"github.com/brewlin/net-protocol/pkg/buffer"
)

type Client struct {
	c *Connection
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
	return &{
		c:c,
		client:fd,
		path:path,

	}
}
func (c *Client)Get(buf string){
	c.c.recv_buf = c.client.Read()
	c.c.parse(c.c)
}