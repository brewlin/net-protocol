package http

import (
	"fmt"
	"log"

	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/pkg/waiter"
	tcpip "github.com/brewlin/net-protocol/protocol"
)

type Connection struct {
	// 客户端连接的socket
	socket tcpip.Endpoint
	// 状态码
	status_code int
	// 接收队列
	recv_buf string
	// HTTP请求
	request *Request
	// HTTP响应
	response *Response
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
func newCon(e tcpip.Endpoint, q *waiter.Queue) *Connection {
	var con Connection
	//创建结构实例
	con.status_code = 0
	con.request_len = 0
	con.socket = e
	con.real_path = ""
	con.recv_state = HTTP_RECV_STATE_WORD1
	con.request = newRequest()
	con.response = newResponse(&con)
	con.recv_buf = ""
	addr, _ := e.GetRemoteAddress()
	log.Println("@application http: new client connection : ", addr)
	con.addr = &addr
	con.waitEntry, con.notifyC = waiter.NewChannelEntry(nil)
	q.EventRegister(&con.waitEntry, waiter.EventIn)
	con.q = q
	return &con

}

//HTTP 请求处理主函数
//从socket中读取数据并解析http请求
//解析请求
//发送响应
//记录请求日志
func (con *Connection) handler() {
	<-con.notifyC
	log.Println("@应用层 http: waiting new event trigger ...")
	fmt.Println("@应用层 http: waiting new event trigger ...")
	for {
		v, _, err := con.socket.Read(con.addr)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				break
			}
			log.Println("@应用层 http:tcp read  got error", err)
			break
		}
		con.recv_buf += string(v)
	}
	fmt.Println("http协议原始数据:")
	fmt.Println(con.recv_buf)
	con.request.parse(con)
	//dispatch the route request
	defaultMux.dispatch(con)
	con.response.send()
}

// 设置状态
func (c *Connection) set_status_code(code int) {
	if c.status_code == 0 {
		c.status_code = code
	}
}

//Write write
func (c *Connection) Write(buf []byte) *tcpip.Error {
	v := buffer.View(buf)
	_, _, err := c.socket.Write(tcpip.SlicePayload(v),
		tcpip.WriteOptions{To: c.addr})
	return err
}

//Read data
func (c *Connection) Read(p []byte) (int, error) {
	buf, _, err := c.socket.Read(c.addr)
	if err != nil {
		return 0, err
	}
	n := copy(p, buf)
	return n, nil

}

//关闭连接
func (c *Connection) Close() {
	if c == nil {
		return
	}
	//释放对应的请求
	c.request = nil
	c.response = nil
	//放放客户端连接中的缓存
	c.recv_buf = ""
	//注销接受队列
	c.q.EventUnregister(&c.waitEntry)
	c.socket.Close()
	//关闭socket连接
	c = nil
}
