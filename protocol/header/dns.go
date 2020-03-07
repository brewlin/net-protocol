package header

import (
	"bytes"
	"encoding/binary"
	"strings"
)


//ADCOUNT question 实体数量占2个字节
//ANCOUNT answer   资源数量占2个字节
//NSCOUNT authority 部分包含的资源数量 2byte
//ARCOUNT additional 部分包含的资源梳理 2byte

//DNSResourceType resource type 表示资源类型
type DNSResourceType uint16

const (
	A  DNSResourceType = iota + 1 // name = hostname value = ipaddress
	NS                            //name = ,value = dns hostname
	MD
	MF
	CNAME //name = hostname
	SOA
	MB
	MG
	MR
	NULL
	WKS
	PTR
	HINFO
	MINFO
	MX
)

const (
	ID = 0
	OP = 2
	QDCOUNT = 4
	ANCOUNT = 6
	NSCOUNT = 8
	ARCOUNT = 10

	DOMAIN = 12
)
//DNSQuestion
type DNSQuestion struct {
	QuestionType  uint16
	QuestionClass uint16
}

//DNSResource ansower,authority,additional
type DNSResource struct {
	Name uint16
	Type DNSResourceType
	Class uint16
	TTL uint32
	RDlen uint16
	RData []byte
	Address string
}


//DNS 报文的封装
type DNS []byte

//GetId
func (d DNS) GetId() uint16 {
	return binary.BigEndian.Uint16(d[ID:OP])
}
//GetQDCount
func (d DNS) GetQDCount()uint16  {
	return binary.BigEndian.Uint16(d[QDCOUNT:QDCOUNT+2])
}
//GetANCount
func (d DNS) GetANCount()uint16{
	return binary.BigEndian.Uint16(d[ANCOUNT:ANCOUNT + 2])
}
//GetNSCount
func (d DNS) GetNSCount() uint16 {
	return binary.BigEndian.Uint16(d[NSCOUNT:NSCOUNT + 2])
}
//GetQACount
func (d DNS) GetARCount () uint16 {
	return binary.BigEndian.Uint16(d[ARCOUNT:ARCOUNT + 2])
}

//GetAnswer
func (d DNS) GetAnswer( ) *[]DNSResource {
	//answer 起始地址
	//asLen := DOMAIN + len(d.getDomain(domain)) + 4
	asLen := DOMAIN + d.GetDomainLen() + 4

	answer := []DNSResource{}
	for i := 0; i < (int(d.GetANCount() + d.GetNSCount() + d.GetARCount())) ;i ++ {
		rs := DNSResource{}
		//判断是不是指针 pointer地址
		if checkP := d[asLen]; checkP >> 6  == 3 {
			//pointer := (d[asLen] & 0x3F << 8) + d[asLen+1]
			rs.Name = binary.BigEndian.Uint16(d[asLen:asLen+2])
			asLen += 2
			rs.Type = DNSResourceType(binary.BigEndian.Uint16(d[asLen:asLen+2]))
			asLen += 2
			rs.Class = binary.BigEndian.Uint16(d[asLen:asLen+2])
			asLen += 2
			rs.TTL = binary.BigEndian.Uint32(d[asLen:asLen+4])
			asLen += 4
			rs.RDlen = binary.BigEndian.Uint16(d[asLen:asLen+2])
			asLen += 2
			rs.RData = d[asLen:asLen+int(rs.RDlen)]
			asLen += int(rs.RDlen)
			answer = append(answer,rs)
		}
	}
	return &answer
}

//Setheader
func (d DNS) Setheader(id uint16){
	d.setID(id)
	d.setFlag(0,0,0,0,1,0,0)
}
//SetCount
func (d DNS) SetCount(qd,an,ns,qa uint16) {
	//SetQdcount
	binary.BigEndian.PutUint16(d[QDCOUNT:], qd)
	//SetAncount
	binary.BigEndian.PutUint16(d[ANCOUNT:] ,an)
	//SetNscount
	binary.BigEndian.PutUint16(d[NSCOUNT:],ns)
	//SetQAcount
	binary.BigEndian.PutUint16(d[ARCOUNT:],qa)
}
//setID
func (d DNS)setID(id uint16){
	//set id
	binary.BigEndian.PutUint16(d[ID:], id)
}
//getDomain
func (d *DNS)getDomain(domain string) []byte {
	var (
		buffer   bytes.Buffer
		segments []string = strings.Split(domain, ".")
	)
	for _, seg := range segments {
		binary.Write(&buffer, binary.BigEndian, byte(len(seg)))
		binary.Write(&buffer, binary.BigEndian, []byte(seg))
	}
	binary.Write(&buffer, binary.BigEndian, byte(0x00))

	return buffer.Bytes()
}
//GetDomainLen 计算domain的长度字节数
func (d DNS) GetDomainLen() (rs int) {
	slen := DOMAIN
	for{
		rs += 1
		if int(d[slen]) == 0 {
			return rs
		}
		rs += int(d[slen])
		slen += int(d[slen]) + 1
	}
}
//SetQuestion query field
//domain url
//qtype type
//qclass class
func (d *DNS)SetQuestion(domain string,qtype,qclass uint16){
	for _,b := range d.getDomain(domain) {
		*d = append((*d),b)
	}
	//d.setDomain(domain)
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
