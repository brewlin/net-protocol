package dns

import (
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/protocol/transport/udp/client"
	_ "github.com/brewlin/net-protocol/pkg/logging"
)
var gid  uint16 =  0x0010

type Endpoint struct {
	ID uint16
	Domain string

	c *client.Client
}
//NewEndpoint
func NewEndpoint(domain string)*Endpoint{
	id := gid + 1
	return &Endpoint{
		Domain:domain,
		c:client.NewClient("8.8.8.8",53),
		ID:id,
	}
}
//Resolve
func (e *Endpoint) Resolve()([]byte,error){
	h := header.DNS(make([]byte,12))
	h.Setheader(e.ID)
	h.SetQdcount(1)
	h.SetAncount(0)
	h.SetNscount(0)
	h.SetQAcount(0)
	h.SetDomain(e.Domain)
	h.SetQuestion(1,1)
	e.c.Connect()
	e.c.Write(h)
	return e.c.Read()


}



