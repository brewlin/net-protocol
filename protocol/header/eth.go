package header

import (
	"encoding/binary"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

//以太网帧头部信息的偏移量
const (
	dstMAC  = 0
	srcMAC  = 6
	ethType = 12
)

//EthernetFields 表示链路层以太网帧的头部
type EthernetFields struct {
	//源地址 (string)
	SrcAddr tcpip.LinkAddress
	//目的地址
	DstAddr tcpip.LinkAddress
	//协议类型
	Type tcpip.NetworkProtocolNumber
}

//Ethernet 以太网数据包的封装
type Ethernet []byte

const (
	//EthernetMinimusize 以太网帧最小的长度
	EtheernetMinimumsize = 14
	//EthernetAddressSize 以太网地址的长度
	EthernetAddressSize = 6
)

//SourceAddress 从帧投不中得到源地址
func (b Ethernet) SourceAddress() tcpip.LinkAddress {
	return tcpip.LinkAddress(b[srcMAC:][:EthernetAddressSize])
}

//DestinationAddress 从帧 头部中得到目的地址
func (b Ethernet) DestinationAddress() tcpip.LinkAddress {
	return tcpip.LinkAddress(b[dstMAC:][:EthernetAddressSize])
}

//Type 从帧头部中得到协议类型
func (b Ethernet) Type() tcpip.NetworkProtocolNumber {
	return tcpip.NetworkProtocolNumber(binary.BigEndian.Uint16(b[ethType:]))
}

//Encode 根据传入的帧头部信息编码成Ethernet二进制形式，注意Ethernet应先分配好内存
func (b Ethernet) Encode(e *EthernetFields) {
	binary.BigEndian.PutUint16(b[ethType:], uint16(e.Type))
	copy(b[srcMAC:][:EthernetAddressSize], e.SrcAddr)
	copy(b[dstMAC:][:EthernetAddressSize], e.DstAddr)
}
