package header

import (
	tcpip "github.com/brewlin/net-protocol/protocol"
)

const (
	//ARPProtocolnumber是arp协议号  为0x0806
	ARPProtocolNumber tcpip.NetworkProtocolNumber = 0x0806
	//arpsize是arp报文在ipv4网络下的长度
	ARPSize = 2 + 2 + 1 + 1 + 2 + 2*6 + 2*4
)

//ARPop 表示ARP的操作码  1 2 3 4
type ARPOp uint16

//RFC 826定义的操作码

const (
	//arp请求
	ARPRequest ARPOp = 1
	//arp应答
	ARPReply ARPOp = 2
)

//ARP报文的封装
type ARP []byte

//从报文中得到硬件类型
func (a ARP) hardwareAddressSpace() uint16 {
	return uint16(a[0])<<8 | uint16(a[1])
}

//从报文中得到协议类型
func (a ARP) protocolAddressSpace() uint16 {
	return uint16(a[2])<<8 | uint16(a[3])
}

//从报文中得到硬件地址长度
func (a ARP) hardwareAddressSize() int {
	return int(a[4])
}

//从报文中得到协议地址长度
func (a ARP) protocolAddressSize() int {
	return int(a[5])
}

//Op从报文中得到arp操作码
func (a ARP) Op() ARPOp {
	return ARPOp(a[6])<<8 | ARPOp(a[7])
}

//SetOp 设置arp操作码
func (a ARP) SetOp(op ARPOp) {
	a[6] = uint8(op >> 8)
	a[7] = uint8(op)
}

//SetIpv4overethernet 设置ipv4网络在以太网中arp报文的硬件和协议信息
func (a ARP) SetIpv4OverEthernet() {
	a[0], a[1] = 0, 1
	a[2], a[3] = 0x08, 0x00
	a[4] = 6
	a[5] = uint8(IPv4AddressSize)
}

//HardwareAddressSender 从报文中得到arp发送方的硬件地址
func (a ARP) HardwareAddressSender() []byte {
	const s = 8
	return a[s : s+6]
}

//ProtocolAddressDender 从报文中得到arp发送方的协议地址，为ipv4地址
func (a ARP) ProtocolAddressSender() []byte {
	const s = 8 + 6
	return a[s : s+4]
}

//HardwareAddressTarget从报文中得到arp目的方硬件地址
func (a ARP) HardwareAddressTarget() []byte {
	const s = 8 + 6 + 4
	return a[s : s+6]
}

//ProtocolAddressTarget从报文中得到ar目的方的协议地址，为ipv4地址
func (a ARP) ProtocolAddressTarget() []byte {
	const s = 8 + 6 + 4 + 6
	return a[s : s+4]
}

//Ivalid检查arp报文是否有效
func (a ARP) IsValid() bool {
	//比arp报文的长度小，返回无效
	if len(a) < ARPSize {
		return false
	}
	const htypeEthernet = 1
	const macSize = 6
	//是否以太网，ipv4，硬件和协议长度都对
	return a.hardwareAddressSpace() == htypeEthernet && a.protocolAddressSpace() == uint16(IPv4ProtocolNumber) && a.hardwareAddressSize() == macSize && a.protocolAddressSize() == IPv4AddressSize
}
