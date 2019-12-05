package socket

import (
	"github.com/brewlin/net-protocol/pkg/waiter"
	tcpip "github.com/brewlin/net-protocol/protocol"
)

//Socket ***client
type Socket interface {
	//Write 向对端写入数据
	Write(buf []byte) error
	//Read 读取单次所有数据包 不等待直接返回
	Read()([]byte,error)
	//Readn 读取n字节
	Readn(p []byte)(int,error)
	//Close
	Close()
	//GetAddr 获取客户端ip地址
	GetAddr()tcpip.Address
	//GetRemoteAddr 获取远程客户端ip地址
	GetRemoteAddr()*tcpip.FullAddress
	//GetQueue 获取接收时间队列
	GetQueue()*waiter.Queue
	GetNotify()chan struct{}
}
