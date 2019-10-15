# websocket protocol api

## @start
初始化阶段和`httpserver`一致、新建httpserver、设置路由、启动监听服务
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