# net-protocol
<p>
<img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/brewlin/net-protocol">
<img alt="GitHub" src="https://img.shields.io/github/license/brewlin/net-protocol">
<img alt="GitHub code size in bytes" src="https://img.shields.io/github/languages/code-size/brewlin/net-protocol">
  
  </p>


基于go 实现链路层、网络层、传输层、应用层 网络协议栈 ，使用虚拟网卡实现

## @application 应用层
- [x] http [docs](./cmd/http.md)
- [ ] websocket

## @transport 传输层
- [x] tcp
- [x] udp
- [x] port 端口机制

## @network 网络层
- [x] icmp
- [x] ipv4
- [x] ipv6

## @link 链路层
- [x] arp
- [x] ethernet

## @物理层
- [x] tun tap 虚拟网卡的实现

