package dns

import (
	_ "github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/protocol/transport/udp/client"
)
var gid  uint16 =  0x0010

type Endpoint struct {
	ID uint16
	Domain string

	//req data
	req *header.DNS
	resp *header.DNS

	answer *[]header.DNSResource

	c *client.Client
}
//NewEndpoint
//support single domain query
func NewEndpoint(domain string)*Endpoint{
	id := gid + 1
	return &Endpoint{
		Domain:domain,
		c:client.NewClient("8.8.8.8",53),
		//c:client.NewClient("114.114.114.114",53),
		ID:id,
	}
}
//Resolve
func (e *Endpoint) Resolve() ( *[]header.DNSResource,error ) {

	h := header.DNS(make([]byte,12))
	h.Setheader(e.ID)
	h.SetCount(1,0,0,0)
	h.SetQuestion(e.Domain,1,1)
	e.req = &h
	return e.sendQuery()
}
//GetResp()
func (e *Endpoint) GetResp() *header.DNS{
	return e.resp
}
//Close close
func (e *Endpoint) Close(){
	e.c.Close()
}



