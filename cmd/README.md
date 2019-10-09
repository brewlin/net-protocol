## docs
- [arp protocol .md](./arp.md)
- [tcp protocol .md](./tcp.md)
- [udp protocol .md](./udp.md)
- [http protocol .md](./http.md)
## example
```
cd tcp/server;
go build;
./server tap1 192.168.1.0/24 192.168.1.1 9111

//mian.go 里的日志是打印到终端
//其他底层的所有详细数据流程日志记录到当前message.txt 中方便观察

cd tcp/client
go build
./client
```
## @demo 协议测试
### arp实验 `cmd/link/arp`
>在网卡的基础上注册了以太网 arp ip等协议，可以了解数据的整个解析和流程
![](./resource/e2.png)

## udp 实验完整的协议栈流程处理 `cmd/udp`
启动网卡，注册相关协议 以太网协议、arp协议、icmp协议、udp协议、初始化端口池

udp的底层实现相对于同为传输层的tcp来说比较简单，无连接、无状态，无需三次握手、四次回收、数据包ack确认、流量控制、窗口移动、确认应答机制、超时重传机制、还有一系列算法进行优化等操作，udp只有简单的收发数据以及占用端口即可
```
cd udp/server;
go build
./server tap1 192.168.1.0/24 192.168.1.1 9000
endpoint.go:89: 注册链路层设备，  new endpoint

//启动网卡 进入事件循环
registration.go:364: @协议栈 stack: register 注册链路层设备LinkEndpointID: 1
stack.go:506: @网卡 stack: 新建网卡对象,并启动网卡事件
nic.go:225: @网卡 nic: 在nic网卡上添加网络层，注册和初始化网络协议  protocol: 2048  addr: 192
nic.go:225: @网卡 nic: 在nic网卡上添加网络层，注册和初始化网络协议  protocol: 2054  addr: 617
stack.go:777: @网卡 stack: 协议解析 nicid: 1  protocol: 2048  addr: 192.168.1.1
ports.go:131: @端口 port: 协议绑定端口 new transport: 17, port: 9000
endpoint.go:190: @链路层 fdbased: dispatch 调度进行事件循环接受物理网卡数据 dispatchLoop
//等待接收数据
endpoint.go:208: @链路层 fdbased: step1 物理网卡接受数据read 42 bytes
endpoint.go:226: @链路层 fdbased: step2 解析以太网协议: [255 255 255 255 255 255 146 234 254 168 251 25 8 6 0 1 8 0 6 4 0 1 146 234 254 168 251 25 10 105 121 88 0 0 0 0 0 0 192 168 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0] 2054 92:ea:fe:a8:fb:19 ff:ff:ff:ff:ff:ff
nic.go:432: @网卡 nic: step3 nic网卡解析以太网协议,分发到对应的 网络层 协议处理 
arp.go:92: @网络层 arp: step1 : 解析arp数据包，包括arp请求和响应
arp.go:103: @网络层 arp: step2 : 解析arp请求
stack.go:777: @网卡 stack: 协议解析 nicid: 1  protocol: 2048  addr: 192.168.1.1
arp.go:115: @网络层 arp: reply: 发送arp回复
endpoint.go:116: @链路层: fdbased 写入网卡数据  0x4c6900
linkaddrcache.go:152: @路由表 route: linkaddrcache 路由表缓存 1:10.105.121.88:0-92:ea:fe:a8:fb:19
endpoint.go:208: @链路层 fdbased: step1 物理网卡接受数据read 47 bytes
endpoint.go:226: @链路层 fdbased: step2 解析以太网协议: [1 1 1 1 1 1 146 234 254 168 251 25 8 0 69 0 0 33 162 77 64 0 64 17 83 20 10 105 121 88 192 168 1 1 157 87 35 40 0 13 182 23 104 101 108 108 111 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0] 2048 92:ea:fe:a8:fb:19 01:01:01:01:01:01
nic.go:432: @网卡 nic: step3 nic网卡解析以太网协议,分发到对应的 网络层 协议处理 
ipv4.go:159: @网络层 ipv4: handlePacket 数据包处理
ipv4.go:195: @网络层 ipv4: handlePacket 分发协议处理，recv ipv4 packet 33 bytes, proto: 0x11
endpoint.go:923: @传输层 udp: handlepacket 从网络层接收到udp数据包 进行处理 
endpoint.go:977: @传输层 udp: recv udp 13 bytes
main.go:150: @main :read and write data:hello
endpoint.go:361: @传输层 udp: 写入udp数据 netProto: 0x800
endpoint.go:561: @传输层 udp: 传输层 udp协议 封装头部信息，发送给网络层  sendudp
ipv4.go:151: @网络层 ipv4: 接受传输层数据 封装ip头部信息，写入链路层网卡数据 send ipv4 packet 33 bytes, proto: 0x11
endpoint.go:116: @链路层: fdbased 写入网卡数据  0x4c6900

```
client调用
```
cd udp/client
go build
./client

main.go:30: send:hello
main.go:37: recv :hello

```

### tcp实验完整的协议栈流程处理 `cmd/tcp`
启动网卡，注册相关协议 以太网协议、arp协议、icmp协议、tcp协议、初始化端口池
```
//tcp 测试基于本地环回网卡，
cd cmd/tcp/server;
go build
./tcp tap1 192.168.1.0/24 192.168.1.1 9000
registration.go:364: @协议栈 stack: register 注册链路层设备LinkEndpointID: 1
stack.go:506: @网卡 stack: 新建网卡对象,并启动网卡事件
nic.go:225: @网卡 nic: 在nic网卡上添加网络层，注册和初始化网络协议  protocol: 2048  addr: 192.168.1.1  peb: 0
nic.go:225: @网卡 nic: 在nic网卡上添加网络层，注册和初始化网络协议  protocol: 2054  addr: 617270  peb: 0
ports.go:131: @端口 port: 协议绑定端口 new transport: 6, port: 9000
endpoint.go:943: @传输层: tcp  connect
main.go:128: Connect is pending...
connect.go:675: @传输层 tcp: send tcp syn segment to 192.168.1.1:9000, seq: 1134465959, ack: 0, rcvWnd: 65535
ipv4.go:151: @网络层 ipv4: 接受传输层数据 封装ip头部信息，写入链路层网卡数据 send ipv4 packet 60 bytes, proto: 0x6
loopback.go:78: @链路层 loopback: 写入网卡数据  0x4bc5c0
nic.go:432: @网卡 nic: step3 nic网卡解析以太网协议,分发到对应的 网络层 协议处理 
ipv4.go:159: @网络层 ipv4: handlePacket 数据包处理
ipv4.go:195: @网络层 ipv4: handlePacket 分发协议处理，recv ipv4 packet 60 bytes, proto: 0x6
endpoint.go:1396: @传输层 tcp:recv tcp syn segment from 192.168.1.1:26913, seq: 1134465959, ack: 0
connect.go:675: @传输层 tcp: send tcp ack|syn segment to 192.168.1.1:26913, seq: 144691071, ack: 1134465960, rcvWnd: 65535
ipv4.go:151: @网络层 ipv4: 接受传输层数据 封装ip头部信息，写入链路层网卡数据 send ipv4 packet 60 bytes, proto: 0x6
loopback.go:78: @链路层 loopback: 写入网卡数据  0x4bc5c0
nic.go:432: @网卡 nic: step3 nic网卡解析以太网协议,分发到对应的 网络层 协议处理 
ipv4.go:159: @网络层 ipv4: handlePacket 数据包处理
ipv4.go:195: @网络层 ipv4: handlePacket 分发协议处理，recv ipv4 packet 60 bytes, proto: 0x6
endpoint.go:1396: @传输层 tcp:recv tcp ack|syn segment from 192.168.1.1:9000, seq: 144691071, ack: 1134465960
connect.go:675: @传输层 tcp: send tcp ack segment to 192.168.1.1:9000, seq: 1134465960, ack: 144691072, rcvWnd: 32768
ipv4.go:151: @网络层 ipv4: 接受传输层数据 封装ip头部信息，写入链路层网卡数据 send ipv4 packet 52 bytes, proto: 0x6
loopback.go:78: @链路层 loopback: 写入网卡数据  0x4bc5c0
nic.go:432: @网卡 nic: step3 nic网卡解析以太网协议,分发到对应的 网络层 协议处理 
ipv4.go:159: @网络层 ipv4: handlePacket 数据包处理
ipv4.go:195: @网络层 ipv4: handlePacket 分发协议处理，recv ipv4 packet 52 bytes, proto: 0x6
endpoint.go:1396: @传输层 tcp:recv tcp ack segment from 192.168.1.1:26913, seq: 1134465960, ack: 144691072
main.go:136: Connected
main.go:108: new conn: 1:192.168.1.1:26913 <nil>
main.go:140: tcp disconnected
connect.go:675: @传输层 tcp: send tcp ack|fin segment to 192.168.1.1:9000, seq: 1134465960, ack: 144691072, rcvWnd: 32768
ipv4.go:151: @网络层 ipv4: 接受传输层数据 封装ip头部信息，写入链路层网卡数据 send ipv4 packet 52 bytes, proto: 0x6
loopback.go:78: @链路层 loopback: 写入网卡数据  0x4bc5c0
nic.go:432: @网卡 nic: step3 nic网卡解析以太网协议,分发到对应的 网络层 协议处理 
ipv4.go:159: @网络层 ipv4: handlePacket 数据包处理
ipv4.go:195: @网络层 ipv4: handlePacket 分发协议处理，recv ipv4 packet 52 bytes, proto: 0x6
endpoint.go:1396: @传输层 tcp:recv tcp ack|fin segment from 192.168.1.1:26913, seq: 1134465960, ack: 144691072
connect.go:675: @传输层 tcp: send tcp ack segment to 192.168.1.1:26913, seq: 144691072, ack: 1134465961, rcvWnd: 32768
ipv4.go:151: @网络层 ipv4: 接受传输层数据 封装ip头部信息，写入链路层网卡数据 send ipv4 packet 52 bytes, proto: 0x6
loopback.go:78: @链路层 loopback: 写入网卡数据  0x4bc5c0
nic.go:432: @网卡 nic: step3 nic网卡解析以太网协议,分发到对应的 网络层 协议处理 
ipv4.go:159: @网络层 ipv4: handlePacket 数据包处理
ipv4.go:195: @网络层 ipv4: handlePacket 分发协议处理，recv ipv4 packet 52 bytes, proto: 0x6
endpoint.go:1396: @传输层 tcp:recv tcp ack segment from 192.168.1.1:9000, seq: 144691072, ack: 1134465961

cd /cmd/tcp/client
go build
./client
```
### 网卡实验`cmd/link/tap`
>提供纯净的虚拟网卡实现，未注册任何协议，可以测试原始网卡的收发数据

