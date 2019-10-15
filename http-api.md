# http protocol api
## @start
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