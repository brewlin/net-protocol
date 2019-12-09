package http

import (
	"github.com/brewlin/net-protocol/internal/socket"
	"log"
	"sync"

	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/pkg/waiter"
	tcpip "github.com/brewlin/net-protocol/protocol"
)

type Connection struct {
	// 客户端连接的socket
	socket socket.Socket
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

	//接受队列缓存区
	buf   buffer.View
	bufmu sync.RWMutex

	q         *waiter.Queue
	waitEntry waiter.Entry
	notifyC   chan struct{}
}

//等待并接受新的连接
func NewCon(e socket.Socket) *Connection {
	var con Connection
	//创建结构实例
	con.status_code = 200
	con.request_len = 0
	con.socket = e
	con.real_path = ""
	con.recv_state = HTTP_RECV_STATE_WORD1
	con.request = newRequest()
	con.response = newResponse(&con)
	con.recv_buf = ""
	log.Println("@application http: new client connection : ", *e.GetRemoteAddr())
	con.addr = e.GetRemoteAddr()
	con.q = e.GetQueue()
	return &con

}
//Close close the connection
func (con *Connection)Close()  {
	con.socket.Close()
}
//Write
func (con *Connection)Write(buf []byte)error{
	return con.socket.Write(buf)
}
//Read 读取单次所有数据包 不等待直接返回
func (con *Connection)Read()([]byte,error){
	return con.socket.Read()
}
//Readn 读取n字节
func (con *Connection)Readn(p []byte)(int,error){
	return con.socket.Readn(p)
}
//HTTP 请求处理主函数
//从socket中读取数据并解析http请求
//解析请求
//发送响应
//记录请求日志
func (con *Connection) handler() {
	log.Println("@应用层 http: waiting new event trigger ...")
	v,_ := con.socket.Read()
	con.recv_buf = string(v)
	log.Println("http协议原始数据:",con.recv_buf)
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
