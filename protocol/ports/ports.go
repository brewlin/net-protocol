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

// Package ports provides PortManager that manages allocating, reserving and releasing ports.
package ports

import (
	"log"
	"math"
	"math/rand"
	"sync"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

const (
	// 临时端口的最小值
	FirstEphemeral = 16000

	anyIPAddress tcpip.Address = ""
)

// 端口的唯一标识: 网络层协议-传输层协议-端口号
type portDescriptor struct {
	network   tcpip.NetworkProtocolNumber
	transport tcpip.TransportProtocolNumber
	port      uint16
}

// 管理端口的对象，由它来保留和释放端口
type PortManager struct {
	mu sync.RWMutex
	// 用一个map接口来保存端口被占用
	allocatedPorts map[portDescriptor]bindAddresses
}

// bindAddresses is a set of IP addresses.
type bindAddresses map[tcpip.Address]struct{}

// isAvailable checks whether an IP address is available to bind to.
func (b bindAddresses) isAvailable(addr tcpip.Address) bool {
	if addr == anyIPAddress {
		return len(b) == 0
	}

	// If all addresses for this portDescriptor are already bound, no
	// address is available.
	if _, ok := b[anyIPAddress]; ok {
		return false
	}

	if _, ok := b[addr]; ok {
		return false
	}
	return true
}

// NewPortManager 新建一个端口管理器
func NewPortManager() *PortManager {
	return &PortManager{allocatedPorts: make(map[portDescriptor]bindAddresses)}
}

// PickEphemeralPort 从端口管理器中随机分配一个端口，并调用testPort来检测是否可用。
func (s *PortManager) PickEphemeralPort(testPort func(p uint16) (bool, *tcpip.Error)) (port uint16, err *tcpip.Error) {
	count := uint16(math.MaxUint16 - FirstEphemeral + 1)
	offset := uint16(rand.Int31n(int32(count)))

	for i := uint16(0); i < count; i++ {
		port = FirstEphemeral + (offset+i)%count
		ok, err := testPort(port)
		if err != nil {
			return 0, err
		}

		if ok {
			return port, nil
		}
	}

	return 0, tcpip.ErrNoPortAvailable
}

// IsPortAvailable tests if the given port is available on all given protocols.
func (s *PortManager) IsPortAvailable(networks []tcpip.NetworkProtocolNumber, transport tcpip.TransportProtocolNumber,
	addr tcpip.Address, port uint16) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isPortAvailableLocked(networks, transport, addr, port)
}

// isPortAvailableLocked 根据参数判断该端口号是否已经被占用了
func (s *PortManager) isPortAvailableLocked(networks []tcpip.NetworkProtocolNumber, transport tcpip.TransportProtocolNumber,
	addr tcpip.Address, port uint16) bool {
	for _, network := range networks {
		desc := portDescriptor{network, transport, port}
		if addrs, ok := s.allocatedPorts[desc]; ok {
			if !addrs.isAvailable(addr) {
				return false
			}
		}
	}
	return true
}

// ReservePort 将端口和IP地址绑定在一起，这样别的程序就无法使用已经被绑定的端口。
// 如果传入的端口不为0，那么会尝试绑定该端口，若该端口没有被占用，那么绑定成功。
// 如果传人的端口等于0，那么就是告诉协议栈自己分配端口，端口管理器就会随机返回一个端口。
func (s *PortManager) ReservePort(networks []tcpip.NetworkProtocolNumber, transport tcpip.TransportProtocolNumber,
	addr tcpip.Address, port uint16) (reservedPort uint16, err *tcpip.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If a port is specified, just try to reserve it for all network
	// protocols.
	if port != 0 {
		if !s.reserveSpecificPort(networks, transport, addr, port) {
			return 0, tcpip.ErrPortInUse
		}
		reservedPort = port
		log.Printf("@端口 port: 协议绑定端口 new transport: %d, port: %d", transport, reservedPort)
		return
	}

	// 随机分配一个未占用的端口
	reservedPort, err = s.PickEphemeralPort(func(p uint16) (bool, *tcpip.Error) {
		return s.reserveSpecificPort(networks, transport, addr, p), nil
	})
	log.Printf("@端口 port: 随机分配端口 协议绑定端口 new transport: %d, port: %d", transport, reservedPort)
	return
}

// reserveSpecificPort 尝试根据协议号和IP地址绑定一个端口
func (s *PortManager) reserveSpecificPort(networks []tcpip.NetworkProtocolNumber, transport tcpip.TransportProtocolNumber,
	addr tcpip.Address, port uint16) bool {
	if !s.isPortAvailableLocked(networks, transport, addr, port) {
		return false
	}

	// Reserve port on all network protocols.
	// 根据给定的网络层协议号（IPV4或IPV6），绑定端口
	for _, network := range networks {
		desc := portDescriptor{network, transport, port}
		m, ok := s.allocatedPorts[desc]
		if !ok {
			m = make(bindAddresses)
			s.allocatedPorts[desc] = m
		}
		// 注册该地址被绑定了
		m[addr] = struct{}{}
	}

	return true
}

// ReleasePort releases the reservation on a port/IP combination so that it can
// be reserved by other endpoints.
// 释放绑定的端口，以便别的程序复用。
func (s *PortManager) ReleasePort(networks []tcpip.NetworkProtocolNumber, transport tcpip.TransportProtocolNumber,
	addr tcpip.Address, port uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 删除绑定关系
	for _, network := range networks {
		desc := portDescriptor{network, transport, port}
		if m, ok := s.allocatedPorts[desc]; ok {
			log.Printf("@端口 port: 释放端口 delete transport: %d, port: %d", transport, port)
			delete(m, addr)
			if len(m) == 0 {
				delete(s.allocatedPorts, desc)
			}
		}
	}
}
