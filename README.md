# net-protocol
基于go 实现链路层、网络层、传输层、应用层 网络协议栈 ，使用虚拟网卡实现

## arp实验 `cmd/link/arp`
```
cd cmd/link/arp;
go build
//启动tap1网卡，并初始化注册arp 以太网协议
sudo ./arp tap1 192.168.1.1/24;
```
### ping 实验
启动 sudo ./arp tap1 192.168.1.1/24:
```
2019/09/03 18:19:46 main.go:28: tap :tap1,cidrName :192.168.1.1/24
2019/09/03 18:19:46 main.go:67: get mac addr: Z¤ΡZ9
2019/09/03 18:19:46 endpoint.go:89: 注册链路层设备，  new endpoint
2019/09/03 18:19:46 registration.go:364: @协议栈 stack: register 注册链路层设备LinkEndpointID: 1
2019/09/03 18:19:46 stack.go:506: @网卡 stack: 新建网卡对象,并启动网卡事件
2019/09/03 18:19:46 nic.go:225: @网卡 nic: 在nic网卡上添加网络层，注册和初始化网络协议  protocol: 2048  addr: 192.168.1.1  peb: 0
2019/09/03 18:19:46 nic.go:225: @网卡 nic: 在nic网卡上添加网络层，注册和初始化网络协议  protocol: 2054  addr: 617270  peb: 0
2019/09/03 18:19:46 endpoint.go:190: @链路层 fdbased: dispatch 调度进行事件循环接受物理网卡数据 dispatchLoop
```
ping 192.168.1.1:
```
2019/09/03 18:03:57 endpoint.go:208: @链路层 fdbased: step1 物理网卡接受数据read 74 bytes
2019/09/03 18:03:57 endpoint.go:226: @链路层 fdbased: step2 解析以太网协议: [46 233 222 153 186 166 46 233 222 153 186 166 8 0 69 0 0 60 161 225 64 0 64 6 83 112 10 105 121 88 192 168 1 1 217 238 0 80 173 245 131 235 0 0 0 0 160 2 114 16 43 63 0 0 2 4 5 180 4 2 8 10 40 66 48 228 0 0 0 0 1 3 3 7 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0] 2048 2e:e9:de:99:ba:a6 2e:e9:de:99:ba:a6
2019/09/03 18:03:57 nic.go:432: @网卡 nic: step3 nic网卡解析以太网协议,分发到对应的 网络层 协议处理 
2019/09/03 18:03:57 ipv4.go:159: @网络层 ipv4: handlePacket 数据包处理
2019/09/03 18:03:57 ipv4.go:195: @网络层 ipv4: handlePacket 分发协议处理，recv ipv4 packet 60 bytes, proto: 0x6

```
## 启动网卡
![](./resource/e1.png)
## 协议解析
![](./resource/e2.png)