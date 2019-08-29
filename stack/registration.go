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

package stack

import (
	"log"
	"sync"

	tcpip "github.com/brewlin/net-protocol/protocol"
	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/pkg/waiter"

	"github.com/brewlin/net-protocol/pkg/sleep"
)

// NetworkEndpointID is the identifier of a network layer protocol endpoint.
// Currently the local address is sufficient because all supported protocols
// (i.e., IPv4 and IPv6) have different sizes for their addresses.
type NetworkEndpointID struct {
	LocalAddress tcpip.Address
}

// TransportEndpointID is the identifier of a transport layer protocol endpoint.
//
// +stateify savable
type TransportEndpointID struct {
	// LocalPort is the local port associated with the endpoint.
	LocalPort uint16

	// LocalAddress is the local [network layer] address associated with
	// the endpoint.
	LocalAddress tcpip.Address

	// RemotePort is the remote port associated with the endpoint.
	RemotePort uint16

	// RemoteAddress it the remote [network layer] address associated with
	// the endpoint.
	RemoteAddress tcpip.Address
}

// ControlType is the type of network control message.
type ControlType int

// The following are the allowed values for ControlType values.
const (
	ControlPacketTooBig ControlType = iota
	ControlPortUnreachable
	ControlUnknown
)

// TransportEndpoint is the interface that needs to be implemented by transport
// protocol (e.g., tcp, udp) endpoints that can handle packets.
type TransportEndpoint interface {
	// HandlePacket is called by the stack when new packets arrive to
	// this transport endpoint.
	HandlePacket(r *Route, id TransportEndpointID, vv buffer.VectorisedView)

	// HandleControlPacket is called by the stack when new control (e.g.,
	// ICMP) packets arrive to this transport endpoint.
	HandleControlPacket(id TransportEndpointID, typ ControlType, extra uint32, vv buffer.VectorisedView)
}

// TransportProtocol is the interface that needs to be implemented by transport
// protocols (e.g., tcp, udp) that want to be part of the networking stack.
type TransportProtocol interface {
	// Number returns the transport protocol number.
	Number() tcpip.TransportProtocolNumber

	// NewEndpoint creates a new endpoint of the transport protocol.
	NewEndpoint(stack *Stack, netProto tcpip.NetworkProtocolNumber, waitQueue *waiter.Queue) (tcpip.Endpoint, *tcpip.Error)

	// MinimumPacketSize returns the minimum valid packet size of this
	// transport protocol. The stack automatically drops any packets smaller
	// than this targeted at this protocol.
	MinimumPacketSize() int

	// ParsePorts returns the source and destination ports stored in a
	// packet of this protocol.
	ParsePorts(v buffer.View) (src, dst uint16, err *tcpip.Error)

	// HandleUnknownDestinationPacket handles packets targeted at this
	// protocol but that don't match any existing endpoint. For example,
	// it is targeted at a port that have no listeners.
	//
	// The return value indicates whether the packet was well-formed (for
	// stats purposes only).
	HandleUnknownDestinationPacket(r *Route, id TransportEndpointID, vv buffer.VectorisedView) bool

	// SetOption allows enabling/disabling protocol specific features.
	// SetOption returns an error if the option is not supported or the
	// provided option value is invalid.
	SetOption(option interface{}) *tcpip.Error

	// Option allows retrieving protocol specific option values.
	// Option returns an error if the option is not supported or the
	// provided option value is invalid.
	Option(option interface{}) *tcpip.Error
}

// TransportDispatcher contains the methods used by the network stack to deliver
// packets to the appropriate transport endpoint after it has been handled by
// the network layer.
type TransportDispatcher interface {
	// DeliverTransportPacket delivers packets to the appropriate
	// transport protocol endpoint.
	DeliverTransportPacket(r *Route, protocol tcpip.TransportProtocolNumber, vv buffer.VectorisedView)

	// DeliverTransportControlPacket delivers control packets to the
	// appropriate transport protocol endpoint.
	DeliverTransportControlPacket(local, remote tcpip.Address, net tcpip.NetworkProtocolNumber, trans tcpip.TransportProtocolNumber, typ ControlType, extra uint32, vv buffer.VectorisedView)
}

// NetworkEndpoint is the interface that needs to be implemented by endpoints
// of network layer protocols (e.g., ipv4, ipv6).
// NetworkEndpoint是需要由网络层协议（例如，ipv4，ipv6）的端点实现的接口。
type NetworkEndpoint interface {
	// DefaultTTL is the default time-to-live value (or hop limit, in ipv6)
	// for this endpoint.
	DefaultTTL() uint8

	// MTU is the maximum transmission unit for this endpoint. This is
	// generally calculated as the MTU of the underlying data link endpoint
	// minus the network endpoint max header length.
	MTU() uint32

	// Capabilities returns the set of capabilities supported by the
	// underlying link-layer endpoint.
	Capabilities() LinkEndpointCapabilities

	// MaxHeaderLength returns the maximum size the network (and lower
	// level layers combined) headers can have. Higher levels use this
	// information to reserve space in the front of the packets they're
	// building.
	MaxHeaderLength() uint16

	// WritePacket writes a packet to the given destination address and
	// protocol.
	WritePacket(r *Route, hdr buffer.Prependable, payload buffer.VectorisedView, protocol tcpip.TransportProtocolNumber, ttl uint8) *tcpip.Error

	// ID returns the network protocol endpoint ID.
	ID() *NetworkEndpointID

	// NICID returns the id of the NIC this endpoint belongs to.
	NICID() tcpip.NICID

	// HandlePacket is called by the link layer when new packets arrive to
	// this network endpoint.
	HandlePacket(r *Route, vv buffer.VectorisedView)

	// Close is called when the endpoint is reomved from a stack.
	Close()
}

// NetworkProtocol is the interface that needs to be implemented by network
// protocols (e.g., ipv4, ipv6) that want to be part of the networking stack.
type NetworkProtocol interface {
	// Number returns the network protocol number.
	Number() tcpip.NetworkProtocolNumber

	// MinimumPacketSize returns the minimum valid packet size of this
	// network protocol. The stack automatically drops any packets smaller
	// than this targeted at this protocol.
	MinimumPacketSize() int

	// ParsePorts returns the source and destination addresses stored in a
	// packet of this protocol.
	ParseAddresses(v buffer.View) (src, dst tcpip.Address)

	// NewEndpoint creates a new endpoint of this protocol.
	NewEndpoint(nicid tcpip.NICID, addr tcpip.Address, linkAddrCache LinkAddressCache, dispatcher TransportDispatcher, sender LinkEndpoint) (NetworkEndpoint, *tcpip.Error)

	// SetOption allows enabling/disabling protocol specific features.
	// SetOption returns an error if the option is not supported or the
	// provided option value is invalid.
	SetOption(option interface{}) *tcpip.Error

	// Option allows retrieving protocol specific option values.
	// Option returns an error if the option is not supported or the
	// provided option value is invalid.
	Option(option interface{}) *tcpip.Error
}

// NetworkDispatcher contains the methods used by the network stack to deliver
// packets to the appropriate network endpoint after it has been handled by
// the data link layer.
type NetworkDispatcher interface {
	// DeliverNetworkPacket finds the appropriate network protocol
	// endpoint and hands the packet over for further processing.
	DeliverNetworkPacket(linkEP LinkEndpoint, dstLinkAddr, srcLinkAddr tcpip.LinkAddress, protocol tcpip.NetworkProtocolNumber, vv buffer.VectorisedView)
}

// LinkEndpointCapabilities is the type associated with the capabilities
// supported by a link-layer endpoint. It is a set of bitfields.
type LinkEndpointCapabilities uint

// The following are the supported link endpoint capabilities.
const (
	CapabilityChecksumOffload LinkEndpointCapabilities = 1 << iota
	CapabilityResolutionRequired
	CapabilitySaveRestore
	CapabilityDisconnectOk
	CapabilityLoopback
)

// LinkEndpoint is the interface implemented by data link layer protocols (e.g.,
// ethernet, loopback, raw) and used by network layer protocols to send packets
// out through the implementer's data link endpoint.
// LinkEndpoint是由数据链路层协议（例如，以太网，环回，原始）实现的接口，并由网络层协议用于通过实施者的数据链路端点发送数据包。
type LinkEndpoint interface {
	// MTU is the maximum transmission unit for this endpoint. This is
	// usually dictated by the backing physical network; when such a
	// physical network doesn't exist, the limit is generally 64k, which
	// includes the maximum size of an IP packet.
	// MTU是此端点的最大传输单位。这通常由支持物理网络决定;
	// 当这种物理网络不存在时，限制通常为64k，其中包括IP数据包的最大大小。
	MTU() uint32

	// Capabilities returns the set of capabilities supported by the
	// endpoint.
	// Capabilities返回链路层端点支持的功能集。
	Capabilities() LinkEndpointCapabilities

	// MaxHeaderLength returns the maximum size the data link (and
	// lower level layers combined) headers can have. Higher levels use this
	// information to reserve space in the front of the packets they're
	// building.
	// MaxHeaderLength 返回数据链接（和较低级别的图层组合）标头可以具有的最大大小。
	// 较高级别使用此信息来保留它们正在构建的数据包前面预留空间。
	MaxHeaderLength() uint16

	// LinkAddress returns the link address (typically a MAC) of the
	// link endpoint.
	// 本地链路层地址
	LinkAddress() tcpip.LinkAddress

	// WritePacket writes a packet with the given protocol through the given
	// route.
	//
	// To participate in transparent bridging, a LinkEndpoint implementation
	// should call eth.Encode with header.EthernetFields.SrcAddr set to
	// r.LocalLinkAddress if it is provided.
	// WritePacket通过给定的路由写入具有给定协议的数据包。
	// 要参与透明桥接，LinkEndpoint实现应调用eth.Encode，并将header.EthernetFields.SrcAddr设置为r.LocalLinkAddress（如果已提供）。
	WritePacket(r *Route, hdr buffer.Prependable, payload buffer.VectorisedView, protocol tcpip.NetworkProtocolNumber) *tcpip.Error

	// Attach attaches the data link layer endpoint to the network-layer
	// dispatcher of the stack.
	// Attach 将数据链路层端点附加到协议栈的网络层调度程序。
	Attach(dispatcher NetworkDispatcher)

	// IsAttached returns whether a NetworkDispatcher is attached to the
	// endpoint.
	// 是否已经添加了网络层调度器
	IsAttached() bool
}

// A LinkAddressResolver is an extension to a NetworkProtocol that
// can resolve link addresses.
type LinkAddressResolver interface {
	// LinkAddressRequest sends a request for the LinkAddress of addr.
	// The request is sent on linkEP with localAddr as the source.
	//
	// A valid response will cause the discovery protocol's network
	// endpoint to call AddLinkAddress.
	LinkAddressRequest(addr, localAddr tcpip.Address, linkEP LinkEndpoint) *tcpip.Error

	// ResolveStaticAddress attempts to resolve address without sending
	// requests. It either resolves the name immediately or returns the
	// empty LinkAddress.
	//
	// It can be used to resolve broadcast addresses for example.
	ResolveStaticAddress(addr tcpip.Address) (tcpip.LinkAddress, bool)

	// LinkAddressProtocol returns the network protocol of the
	// addresses this this resolver can resolve.
	LinkAddressProtocol() tcpip.NetworkProtocolNumber
}

// A LinkAddressCache caches link addresses.
type LinkAddressCache interface {
	// CheckLocalAddress determines if the given local address exists, and if it
	// does not exist.
	CheckLocalAddress(nicid tcpip.NICID, protocol tcpip.NetworkProtocolNumber, addr tcpip.Address) tcpip.NICID

	// AddLinkAddress adds a link address to the cache.
	AddLinkAddress(nicid tcpip.NICID, addr tcpip.Address, linkAddr tcpip.LinkAddress)

	// GetLinkAddress looks up the cache to translate address to link address (e.g. IP -> MAC).
	// If the LinkEndpoint requests address resolution and there is a LinkAddressResolver
	// registered with the network protocol, the cache attempts to resolve the address
	// and returns ErrWouldBlock. Waker is notified when address resolution is
	// complete (success or not).
	//
	// If address resolution is required, ErrNoLinkAddress and a notification channel is
	// returned for the top level caller to block. Channel is closed once address resolution
	// is complete (success or not).
	GetLinkAddress(nicid tcpip.NICID, addr, localAddr tcpip.Address, protocol tcpip.NetworkProtocolNumber, w *sleep.Waker) (tcpip.LinkAddress, <-chan struct{}, *tcpip.Error)

	// RemoveWaker removes a waker that has been added in GetLinkAddress().
	RemoveWaker(nicid tcpip.NICID, addr tcpip.Address, waker *sleep.Waker)
}

// TransportProtocolFactory functions are used by the stack to instantiate
// transport protocols.
type TransportProtocolFactory func() TransportProtocol

// NetworkProtocolFactory provides methods to be used by the stack to
// instantiate network protocols.
type NetworkProtocolFactory func() NetworkProtocol

var (
	// 传输层协议的注册储存结构
	transportProtocols = make(map[string]TransportProtocolFactory)
	// 网络层协议的注册存储结构
	networkProtocols = make(map[string]NetworkProtocolFactory)

	linkEPMu           sync.RWMutex
	nextLinkEndpointID tcpip.LinkEndpointID = 1
	linkEndpoints                           = make(map[tcpip.LinkEndpointID]LinkEndpoint)
)

// RegisterTransportProtocolFactory registers a new transport protocol factory
// with the stack so that it becomes available to users of the stack. This
// function is intended to be called by init() functions of the protocols.
// RegisterTransportProtocolFactory 在协议栈中注册一个新的传输协议工厂，
// 以便协议栈可以使用它。此函数在由协议的init函数中使用。
func RegisterTransportProtocolFactory(name string, p TransportProtocolFactory) {
	transportProtocols[name] = p
}

// RegisterNetworkProtocolFactory registers a new network protocol factory with
// the stack so that it becomes available to users of the stack. This function
// is intended to be called by init() functions of the protocols.
// RegisterNetworkProtocolFactory 在协议栈中注册一个新的网络协议工厂，
// 以便协议栈可以使用它。此函数在由协议的init函数中使用。
func RegisterNetworkProtocolFactory(name string, p NetworkProtocolFactory) {
	networkProtocols[name] = p
}

// RegisterLinkEndpoint register a link-layer protocol endpoint and returns an
// ID that can be used to refer to it.
// 注册一个链路层设备
func RegisterLinkEndpoint(linkEP LinkEndpoint) tcpip.LinkEndpointID {
	linkEPMu.Lock()
	defer linkEPMu.Unlock()

	v := nextLinkEndpointID
	nextLinkEndpointID++

	linkEndpoints[v] = linkEP
	log.Println("@协议栈 stack: register 注册链路层设备LinkEndpointID:", v)

	return v
}

// FindLinkEndpoint finds the link endpoint associated with the given ID.
// 根据ID找到网卡设备
func FindLinkEndpoint(id tcpip.LinkEndpointID) LinkEndpoint {
	linkEPMu.RLock()
	defer linkEPMu.RUnlock()

	return linkEndpoints[id]
}
