package header

import (
	"github.com/brewlin/net-protocol/tcpip"
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
