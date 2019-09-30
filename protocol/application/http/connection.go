package http

import (
	"log"

	"github.com/brewlin/net-protocol/pkg/waiter"
	tcpip "github.com/brewlin/net-protocol/protocol"
)

type connection struct {
	// 客户端连接的socket
	socket tcpip.Endpoint
	// 状态码
	status_code int
	// 接收队列
	recv_buf string
	// HTTP请求
	request *http_request
	// HTTP响应
	response *http_response
	// 接收状态
	recv_state http_recv_state
	// 客户端地址信息
	addr *tcpip.FullAddress
	// 请求长度
	request_len int
	// 请求文件的真实路径
	real_path string

	q         *waiter.Queue
	waitEntry waiter.Entry
	notifyC   chan struct{}
}

//等待并接受新的连接
func newCon(e tcpip.Endpoint, q *waiter.Queue) *connection {
	var con *connection
	//创建结构实例
	con.status_code = 0
	con.request_len = 0
	con.socket = e
	con.real_path = ""
	con.recv_state = HTTP_RECV_STATE_WORD1
	con.request = newRequest()
	con.response = newResponse()
	con.recv_buf = ""
	addr, _ := e.GetRemoteAddress()
	con.addr = &addr
	con.waitEntry, con.notifyC = waiter.NewChannelEntry(nil)
	q.EventRegister(&con.waitEntry, waiter.EventIn)
	con.q = q
	return con

}

//HTTP 请求处理主函数
//从socket中读取数据并解析http请求
//解析请求
//发送响应
//记录请求日志
func (con *connection) handler() {
	for {
		v, cc, err := con.socket.Read(con.addr)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				log.Println("@应用层 http: waiting new event trigger ...")
				<-con.notifyC
				continue
			}
			log.Println("@应用层 http:tcp read  got error", err)
			break
		}
		log.Println("@应用层 http: recv ", v, cc)
		a, b, er := con.socket.Write(tcpip.SlicePayload(v), tcpip.WriteOptions{To: con.addr})
		log.Println("@应用层 http: write to client res: ", a, b, er)
	}
}

//关闭连接
func (c *connection) close() {
	if c == nil {
		return
	}
	//释放对应的请求
	c.request = nil
	c.response = nil
	//放放客户端连接中的缓存
	c.recv_buf = ""
	//关闭socket连接
	c = nil
	//注销接受队列
	c.q.EventUnregister(&c.waitEntry)
}