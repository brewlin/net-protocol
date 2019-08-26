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
	"sync"

	tcpip "github.com/brewlin/net-protocol/protocol"
	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/protocol/header"
)

// 网络层协议号和传输层协议号的组合，当作分流器的key值
type protocolIDs struct {
	network   tcpip.NetworkProtocolNumber
	transport tcpip.TransportProtocolNumber
}

// transportEndpoints manages all endpoints of a given protocol. It has its own
// mutex so as to reduce interference between protocols.
// transportEndpoints 管理给定协议的所有端点。
type transportEndpoints struct {
	mu        sync.RWMutex
	endpoints map[TransportEndpointID]TransportEndpoint
}

// transportDemuxer demultiplexes packets targeted at a transport endpoint
// (i.e., after they've been parsed by the network layer). It does two levels
// of demultiplexing: first based on the network and transport protocols, then
// based on endpoints IDs.
// transportDemuxer 解复用针对传输端点的数据包（即，在它们被网络层解析之后）。
// 它执行两级解复用：首先基于网络层协议和传输协议，然后基于端点ID。
type transportDemuxer struct {
	protocol map[protocolIDs]*transportEndpoints
}

// 新建一个分流器
func newTransportDemuxer(stack *Stack) *transportDemuxer {
	d := &transportDemuxer{protocol: make(map[protocolIDs]*transportEndpoints)}

	// Add each network and transport pair to the demuxer.
	for netProto := range stack.networkProtocols {
		for proto := range stack.transportProtocols {
			d.protocol[protocolIDs{netProto, proto}] = &transportEndpoints{endpoints: make(map[TransportEndpointID]TransportEndpoint)}
		}
	}

	return d
}

// registerEndpoint registers the given endpoint with the dispatcher such that
// packets that match the endpoint ID are delivered to it.
// registerEndpoint 向分发器注册给定端点，以便将与端点ID匹配的数据包传递给它。
func (d *transportDemuxer) registerEndpoint(netProtos []tcpip.NetworkProtocolNumber, protocol tcpip.TransportProtocolNumber, id TransportEndpointID, ep TransportEndpoint) *tcpip.Error {
	for i, n := range netProtos {
		if err := d.singleRegisterEndpoint(n, protocol, id, ep); err != nil {
			d.unregisterEndpoint(netProtos[:i], protocol, id)
			return err
		}
	}

	return nil
}

func (d *transportDemuxer) singleRegisterEndpoint(netProto tcpip.NetworkProtocolNumber, protocol tcpip.TransportProtocolNumber, id TransportEndpointID, ep TransportEndpoint) *tcpip.Error {
	eps, ok := d.protocol[protocolIDs{netProto, protocol}]
	if !ok {
		return nil
	}

	eps.mu.Lock()
	defer eps.mu.Unlock()

	if _, ok := eps.endpoints[id]; ok {
		return tcpip.ErrPortInUse
	}

	eps.endpoints[id] = ep

	return nil
}

// unregisterEndpoint unregisters the endpoint with the given id such that it
// won't receive any more packets.
// unregisterEndpoint 使用给定的id注销端点，使其不再接收任何数据包。
func (d *transportDemuxer) unregisterEndpoint(netProtos []tcpip.NetworkProtocolNumber, protocol tcpip.TransportProtocolNumber, id TransportEndpointID) {
	for _, n := range netProtos {
		if eps, ok := d.protocol[protocolIDs{n, protocol}]; ok {
			eps.mu.Lock()
			delete(eps.endpoints, id)
			eps.mu.Unlock()
		}
	}
}

// deliverPacket attempts to deliver the given packet. Returns true if it found
// an endpoint, false otherwise.
// 根据传输层的id来找到对应的传输端，再将数据包交给这个传输端处理
func (d *transportDemuxer) deliverPacket(r *Route, protocol tcpip.TransportProtocolNumber, vv buffer.VectorisedView, id TransportEndpointID) bool {
	// 先看看分流器里有没有注册相关协议端，如果没有则返回false
	eps, ok := d.protocol[protocolIDs{r.NetProto, protocol}]
	if !ok {
		return false
	}

	// 从 eps 中找符合 id 的传输端
	eps.mu.RLock()
	ep := d.findEndpointLocked(eps, vv, id)
	eps.mu.RUnlock()

	// Fail if we didn't find one.
	if ep == nil {
		// UDP packet could not be delivered to an unknown destination port.
		if protocol == header.UDPProtocolNumber {
			r.Stats().UDP.UnknownPortErrors.Increment()
		}
		return false
	}

	// Deliver the packet.
	// 找到传输端到，让它来处理数据包
	ep.HandlePacket(r, id, vv)

	return true
}

// deliverControlPacket attempts to deliver the given control packet. Returns
// true if it found an endpoint, false otherwise.
func (d *transportDemuxer) deliverControlPacket(net tcpip.NetworkProtocolNumber, trans tcpip.TransportProtocolNumber, typ ControlType, extra uint32, vv buffer.VectorisedView, id TransportEndpointID) bool {
	eps, ok := d.protocol[protocolIDs{net, trans}]
	if !ok {
		return false
	}

	// Try to find the endpoint.
	eps.mu.RLock()
	ep := d.findEndpointLocked(eps, vv, id)
	eps.mu.RUnlock()

	// Fail if we didn't find one.
	if ep == nil {
		return false
	}

	// Deliver the packet.
	ep.HandleControlPacket(id, typ, extra, vv)

	return true
}

// 根据传输层id来找到相应的传输层端
func (d *transportDemuxer) findEndpointLocked(eps *transportEndpoints, vv buffer.VectorisedView, id TransportEndpointID) TransportEndpoint {
	// Try to find a match with the id as provided.
	// 从 endpoints 中看有没有匹配到id的传输层端
	if ep := eps.endpoints[id]; ep != nil {
		return ep
	}

	// Try to find a match with the id minus the local address.
	nid := id
	// 如果上面的 endpoints 没有找到，那么去掉本地ip地址，看看有没有相应的传输层端
	// 因为有时候传输层监听的时候没有绑定本地ip，也就是 any address，此时的 LocalAddress
	// 为空。
	nid.LocalAddress = ""
	if ep := eps.endpoints[nid]; ep != nil {
		return ep
	}

	// Try to find a match with the id minus the remote part.
	nid.LocalAddress = id.LocalAddress
	nid.RemoteAddress = ""
	nid.RemotePort = 0
	if ep := eps.endpoints[nid]; ep != nil {
		return ep
	}

	// Try to find a match with only the local port.
	nid.LocalAddress = ""
	return eps.endpoints[nid]
}
