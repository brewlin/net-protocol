package dns

import (
	"github.com/brewlin/net-protocol/protocol/header"
	"strconv"
	"strings"
)

//parseResp
//解析响应
func (e *Endpoint) parseResp() (*[]header.DNSResource,error){
	rsp,err := e.c.Read()
	if err != nil {
		return nil,err
	}
	p := header.DNS(rsp)
	e.resp = &p
	e.answer = p.GetAnswer( )
	return e.parseAnswer()
}

func (e *Endpoint) parseAnswer()(*[]header.DNSResource,error){
	for i := 0; i < len(*e.answer) ; i++ {
		switch (*e.answer)[i].Type {
		case header.A:
			(*e.answer)[i].Address = e.parseAName((*e.answer)[i].RData)
		case header.CNAME:
			(*e.answer)[i].Address = e.parseCName((*e.answer)[i].RData)
		}
	}
	return e.answer,nil
}
func (e *Endpoint)parseAName(rd []byte) string {
	res := []string{}
	for _,v := range rd {
		res = append(res,strconv.Itoa(int(v)))
	}
	return strings.Join(res,".")
}

func (e *Endpoint)parseCName(rd []byte) (res string) {

	for{
		l := int(rd[0])
		if l >= len(rd){
			res += ".com"
			return
		}
		rd = rd[1:]
		res += string(rd[0:l])
		rd = rd[l:]
		if len(rd) == 0 {
			return
		}

	}
}
