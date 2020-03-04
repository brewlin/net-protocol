package header

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

//DNSop 表示dns header 操作码
type DNSop uint16

//1(QR)+4(OPCODE)+1(AA)+1(TC)+1(RD)+1(RA)+3(Z)+4(RCODE)
const (
	//dns请求
	DNSRequest DNSop = 0
	//dns应答
	DNSReply DNSop = 1

	//opcode 标准请求
	DNSOpcodeQ DNSop = 0 << 1
	//opcode 反转查询
	DNSOpcodeR DNSop = 1 << 1
	//opcode 服务器状态查询
	DNSOpcodeS DNSop = 2 << 1

	//权威应答
	DNSAaN DNSop = 0 << 5
	DNSAaY DNSop = 1 << 5

	//截断
	DNSTcN DNSop = 0 << 6
	DNSTcY DNSop = 1 << 6

	//期望递归
	DNSRdN DNSop = 0 << 7
	DNSRdY DNSop = 1 << 7

	//递归可用性
	DNSRaN DNSop = 0 << 8
	DNSRaY DNSop = 1 << 8

	//保留备用 占3位 + 3

	//RCODE 响应code 占4位
	DNSRocdeN   DNSop = 0 << 12
	DNSRocdeF   DNSop = 1 << 12
	DNSRocdeS   DNSop = 2 << 12
	DNSRocdeNa  DNSop = 3 << 12
	DNSRocdeNot DNSop = 4 << 12
	DNSRocdeRef DNSop = 5 << 12
)

//ADCOUNT question 实体数量占2个字节
//ANCOUNT answer   资源数量占2个字节
//NSCOUNT authority 部分包含的资源数量 2byte
//ARCOUNT additional 部分包含的资源梳理 2byte

//DNS 报文的封装
type DNS []byte
