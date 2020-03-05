// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package arp implements the ARP network protocol. It is used to resolve
// IPv4 addresses into link-local MAC addresses, and advertises IPv4
// addresses of its stack with the local network.
//
// To use it in the networking stack, pass arp.ProtocolName as one of the
// network protocols when calling stack.New. Then add an "arp" address to
// every NIC on the stack that should respond to ARP requests. That is:
//
//	if err := s.AddAddress(1, arp.ProtocolNumber, "arp"); err != nil {
//		// handle err
//	}
package arp

import (
	"log"

	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/stack"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

const (
	// ProtocolName is the string representation of the ARP protocol name.
	ProtocolName = "arp"

	// ProtocolNumber is the ARP protocol number.
	ProtocolNumber = header.ARPProtocolNumber

	// ProtocolAddress is the address expected by the ARP endpoint.
	ProtocolAddress = tcpip.Address("arp")
)

// endpoint implements stack.NetworkEndpoint.
// 表示arp端
type endpoint struct {
	nicid         tcpip.NICID
	addr          tcpip.Address
	linkEP        stack.LinkEndpoint
	linkAddrCache stack.LinkAddressCache
}

// DefaultTTL is unused for ARP. It implements stack.NetworkEndpoint.
func (e *endpoint) DefaultTTL() uint8 {
	return 0
}

func (e *endpoint) MTU() uint32 {
	lmtu := e.linkEP.MTU()
	return lmtu - uint32(e.MaxHeaderLength())
}

func (e *endpoint) NICID() tcpip.NICID {
	return e.nicid
}

func (e *endpoint) Capabilities() stack.LinkEndpointCapabilities {
	return e.linkEP.Capabilities()
}

func (e *endpoint) ID() *stack.NetworkEndpointID {
	return &stack.NetworkEndpointID{ProtocolAddress}
}

func (e *endpoint) MaxHeaderLength() uint16 {
	return e.linkEP.MaxHeaderLength() + header.ARPSize
}

func (e *endpoint) Close() {}

func (e *endpoint) WritePacket(r *stack.Route, hdr buffer.Prependable, payload buffer.VectorisedView, protocol tcpip.TransportProtocolNumber, ttl uint8) *tcpip.Error {
	return tcpip.ErrNotSupported
}

// arp数据包的处理，包括arp请求和响应
func (e *endpoint) HandlePacket(r *stack.Route, vv buffer.VectorisedView) {
	log.Println("@网络层 arp: step1 : 解析arp数据包，包括arp请求和响应")
	v := vv.First()
	h := header.ARP(v)
	if !h.IsValid() {
		return
	}

	// 判断操作码类型
	switch h.Op() {
	case header.ARPRequest:
		// 如果是arp请求
		log.Println("@网络层 arp: step2 : 解析arp请求")
		localAddr := tcpip.Address(h.ProtocolAddressTarget())
		if e.linkAddrCache.CheckLocalAddress(e.nicid, header.IPv4ProtocolNumber, localAddr) == 0 {
			return // we have no useful answer, ignore the request
		}
		hdr := buffer.NewPrependable(int(e.linkEP.MaxHeaderLength()) + header.ARPSize)
		pkt := header.ARP(hdr.Prepend(header.ARPSize))
		pkt.SetIpv4OverEthernet()
		pkt.SetOp(header.ARPReply)
		copy(pkt.HardwareAddressSender(), r.LocalLinkAddress[:])
		copy(pkt.HardwareAddressTarget(),h.HardwareAddressSender())
		copy(pkt.ProtocolAddressSender(), h.ProtocolAddressTarget())
		copy(pkt.ProtocolAddressTarget(), h.ProtocolAddressSender())
		log.Println("@网络层 arp: reply: 发送arp回复",h.ProtocolAddressSender(),h.HardwareAddressTarget())
		e.linkEP.WritePacket(r, hdr, buffer.VectorisedView{}, ProtocolNumber)
		// 注意这里的 fallthrough 表示需要继续执行下面分支的代码
		// 当收到 arp 请求需要添加到链路地址缓存中
		fallthrough // also fill the cache from requests
	case header.ARPReply:
		log.Println("@网络层 arp: step2 : 解析arp回复")
		// 这里记录ip和mac对应关系，也就是arp表
		addr := tcpip.Address(h.ProtocolAddressSender())
		linkAddr := tcpip.LinkAddress(h.HardwareAddressSender())
		e.linkAddrCache.AddLinkAddress(e.nicid, addr, linkAddr)
	}
}

// protocol implements stack.NetworkProtocol and stack.LinkAddressResolver.
// 实现了 stack.NetworkProtocol 和 stack.LinkAddressResolver 两个接口
type protocol struct {
}

func (p *protocol) Number() tcpip.NetworkProtocolNumber { return ProtocolNumber }
func (p *protocol) MinimumPacketSize() int              { return header.ARPSize }

func (*protocol) ParseAddresses(v buffer.View) (src, dst tcpip.Address) {
	h := header.ARP(v)
	return tcpip.Address(h.ProtocolAddressSender()), ProtocolAddress
}

func (p *protocol) NewEndpoint(nicid tcpip.NICID, addr tcpip.Address, linkAddrCache stack.LinkAddressCache, dispatcher stack.TransportDispatcher, sender stack.LinkEndpoint) (stack.NetworkEndpoint, *tcpip.Error) {
	if addr != ProtocolAddress {
		return nil, tcpip.ErrBadLocalAddress
	}
	return &endpoint{
		nicid:         nicid,
		addr:          addr,
		linkEP:        sender,
		linkAddrCache: linkAddrCache,
	}, nil
}

// LinkAddressProtocol implements stack.LinkAddressResolver.
func (*protocol) LinkAddressProtocol() tcpip.NetworkProtocolNumber {
	return header.IPv4ProtocolNumber
}

// LinkAddressRequest implements stack.LinkAddressResolver.
func (*protocol) LinkAddressRequest(addr, localAddr tcpip.Address, linkEP stack.LinkEndpoint) *tcpip.Error {
	r := &stack.Route{
		RemoteLinkAddress: broadcastMAC,
	}

	hdr := buffer.NewPrependable(int(linkEP.MaxHeaderLength()) + header.ARPSize)
	h := header.ARP(hdr.Prepend(header.ARPSize))
	h.SetIpv4OverEthernet()
	h.SetOp(header.ARPRequest)
	copy(h.HardwareAddressSender(), linkEP.LinkAddress())
	copy(h.ProtocolAddressSender(), localAddr)
	copy(h.ProtocolAddressTarget(), addr)

	return linkEP.WritePacket(r, hdr, buffer.VectorisedView{}, ProtocolNumber)
}

// ResolveStaticAddress implements stack.LinkAddressResolver.
func (*protocol) ResolveStaticAddress(addr tcpip.Address) (tcpip.LinkAddress, bool) {
	if addr == "\xff\xff\xff\xff" {
		return broadcastMAC, true
	}
	return "", false
}

// SetOption implements NetworkProtocol.
func (p *protocol) SetOption(option interface{}) *tcpip.Error {
	return tcpip.ErrUnknownProtocolOption
}

// Option implements NetworkProtocol.
func (p *protocol) Option(option interface{}) *tcpip.Error {
	return tcpip.ErrUnknownProtocolOption
}

var broadcastMAC = tcpip.LinkAddress([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

func init() {
	stack.RegisterNetworkProtocolFactory(ProtocolName, func() stack.NetworkProtocol {
		return &protocol{}
	})
}
