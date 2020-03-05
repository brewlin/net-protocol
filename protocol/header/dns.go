package header

import (
	"bytes"
	"encoding/binary"
	"strings"
)

//DNSResourceType resource type 表示资源类型
type DNSResourceType uint16

//DNSop 表示dns header 操作码
type DNSop uint16

//ADCOUNT question 实体数量占2个字节
//ANCOUNT answer   资源数量占2个字节
//NSCOUNT authority 部分包含的资源数量 2byte
//ARCOUNT additional 部分包含的资源梳理 2byte


const (
	ID = 0
	OP = 2
	QDCOUNT = 4
	ANCOUNT = 6
	NSCOUNT = 8
	QACOUNT = 10

	DOMAIN = 12
)

type DNSQuestion struct {
	QuestionType  uint16
	QuestionClass uint16
}

//DNS 报文的封装
type DNS []byte

//Setheader
func (d DNS) Setheader(id uint16){
	d.setID(id)
	d.setFlag(0,0,0,0,1,0,0)
}
//SetID
func (d DNS)setID(id uint16){
	//set id
	binary.BigEndian.PutUint16(d[ID:], id)
}
//SetQdcount
func (d DNS)SetQdcount(qd uint16){

	binary.BigEndian.PutUint16(d[QDCOUNT:], qd)
}
//SetAncount
func (d DNS)SetAncount(an uint16){
	binary.BigEndian.PutUint16(d[ANCOUNT:] ,an)
}
//SetNscount
func (d DNS)SetNscount(ns uint16){
	binary.BigEndian.PutUint16(d[NSCOUNT:],ns)
}
//SetQAcount
func (d DNS)SetQAcount(qa uint16){
	binary.BigEndian.PutUint16(d[QACOUNT:],qa)
}
//SetDomain
func (d *DNS)SetDomain(domain string) {
	var (
		buffer   bytes.Buffer
		segments []string = strings.Split(domain, ".")
	)
	binary.Write(&buffer,binary.BigEndian,*d)

	for _, seg := range segments {
		binary.Write(&buffer, binary.BigEndian, byte(len(seg)))
		binary.Write(&buffer, binary.BigEndian, []byte(seg))
	}
	binary.Write(&buffer, binary.BigEndian, byte(0x00))

	*d = buffer.Bytes()
	//return buffer.Bytes()
}
func (d *DNS)SetQuestion(qtype,qclass uint16){
	q := DNSQuestion{
		QuestionType:  qtype,
		QuestionClass: qclass,
	}
	var buffer bytes.Buffer
	binary.Write(&buffer,binary.BigEndian,*d)
	binary.Write(&buffer,binary.BigEndian,q)
	*d = buffer.Bytes()
}

//SetFlag
//QR 表示请求还是响应
//OPCODE 1表示反转查询；2表示服务器状态查询。3~15目前保留，以备将来使用
//AA 表示响应的服务器是否是权威DNS服务器。只在响应消息中有效。
//TC 指示消息是否因为传输大小限制而被截断
//RD 该值在请求消息中被设置，响应消息复用该值。如果被设置，表示希望服务器递归查询。但服务器不一定支持递归查询
//RA 。该值在响应消息中被设置或被清除，以表明服务器是否支持递归查询。
//Z 保留备用
//RCODE: 该值在响应消息中被设置。取值及含义如下：
//0：No error condition，没有错误条件；
//1：Format error，请求格式有误，服务器无法解析请求；
//2：Server failure，服务器出错。
//3：Name Error，只在权威DNS服务器的响应中有意义，表示请求中的域名不存在。
//4：Not Implemented，服务器不支持该请求类型。
//5：Refused，服务器拒绝执行请求操作。
//6~15：保留备用
func (d DNS) setFlag(QR uint16, OPCODE uint16, AA uint16, TC uint16, RD uint16, RA uint16, RCODE uint16) {
	//set flag
	op :=  QR<<15 + OPCODE<<11 + AA<<10 + TC<<9 + RD<<8 + RA<<7 + RCODE
	binary.BigEndian.PutUint16(d[OP:],op)
}
