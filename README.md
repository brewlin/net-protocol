# net-protocol
基于go 实现链路层、网络层、传输层、应用层 网络协议栈 ，使用虚拟网卡实现

## arp实验 `cmd/link/arp`
>在网卡的基础上注册了以太网 arp ip等协议，可以了解数据的整个解析和流程
![](./resource/e2.png)

## 网卡实验 `cmd/link/tap`
>提供纯净的虚拟网卡实现，未注册任何协议，可以测试原始网卡的收发数据

