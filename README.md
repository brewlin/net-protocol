# net-protocol
<p>
<img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/brewlin/net-protocol">
<img alt="GitHub" src="https://img.shields.io/github/license/brewlin/net-protocol">
<img alt="GitHub code size in bytes" src="https://img.shields.io/github/languages/code-size/brewlin/net-protocol">
  <img alt="GitHub release (latest by date)" src="https://img.shields.io/github/v/release/brewlin/net-protocol">
  </p>


基于go 实现链路层、网络层、传输层、应用层 网络协议栈 ，使用虚拟网卡实现
## @docs
```
相关md文档在cmd目录下，以及相关协议的demo测试
```
`./cmd/*.md`
## @application 应用层
- [x] http [docs](./cmd/http.md)
- [x] websocket [docs](./cmd/websocket.md)

## @transport 传输层
- [x] tcp [docs](./cmd/tcp.md)
- [x] udp [docs](./cmd/udp.md)
- [x] port 端口机制

## @network 网络层
- [x] icmp
- [x] ipv4
- [x] ipv6

## @link 链路层
- [x] arp [docs](./cmd/arp.md)
- [x] ethernet

## @物理层
- [x] tun tap 虚拟网卡的实现

## 协议相关api
### 1.应用层相关协议
> 应用层暂时只实现了http、websocket、等文本协议。都基于tcp实现，对tcp等进行二次封装

[`http`](./cmd/http.md):
```
//新建一个初始化server，（底层 会创建一个tap网卡并注册 路由，arp缓存等），初始化端口机制，添加9502到端口表
serv := http.NewHTTP("tap1", "192.168.1.0/24", "192.168.1.1", "9502")
//添加路由，当对应请求到来时，分发到自定义回调函数中处理
serv.HandleFunc("/", func(request *http.Request, response *http.Response) 
//赋值给将要发送响应给客户端的 buf
Response.End("string");
//启动监听网卡、启动tcp、启动dispatch 分发事件并阻塞 等待client连接。。
serv.ListenAndServ()
```
[`websocket`](./cmd/websocket):初始化阶段和`httpserver`一致、新建httpserver、设置路由、启动监听服务
```
// r *http.Request, w *http.Response
// Upgrade 为校验websocket协议， 并将http协议升级为websocket协议。并接管http协议流程，直接进行tcp通讯保持连接，
c, err := websocket.Upgrade(r, w)

	//循环处理数据，接受数据，然后返回
	for {
    //读取客户端数据，该方法一直阻塞直到收到客户端数据，会触发通道取消阻塞
		message, err := c.ReadData()
    //发送数据给客户端，封装包头包体，调用tcpWrite 封装tcp包头，写入网络层 封装ip包头、写入链路层 封装以太网包头、写入网卡
		c.SendData([]byte("hello"))
	}
```

### 2.传输层相关协议
tcp

udp

port端口机制

