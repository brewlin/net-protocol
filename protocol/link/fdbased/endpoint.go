package fdbased

import (
	"log"
	"syscall"

	"github.com/brewlin/net-protocol/stack"
	"github.com/brewlin/net-protocol"
	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/protocol/link/rawfile"
)

// 从NIC读取数据的多级缓存配置
var BufConfig = []int{128, 256, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768}

//负责底层网卡的io读写以及数据分发
type endpoint struct {
	//发送和接受数据的文件描述符
	fd int

	//单个帧的最大长度
	mtu uint32

	//以太网头部长度
	hdrSize int

	//网卡地址
	addr tcpip.LinkAddress

	//网卡的能力
	caps stack.LinkEndpointCapabilities

	//关闭发送执行管道
	closed func(*tcpip.Error)

	iovecs     []syscall.Iovec
	views      []buffer.View
	dispatcher stack.NetworkDispatcher

	//handleLocal 指示发往自身的数据包是由内部 github.com/brewlin/net-protocol 协议栈处理(true) 还是转发到FD端点（false)
	handleLocal bool
}

//创建fdbase端的一些选项参数
type Options struct {
	FD                 int
	MTU                uint32
	CloseFunc          func(*tcpip.Error)
	Address            tcpip.LinkAddress
	ResolutionRequired bool
	SaveRestore        bool
	ChecksumOffload    bool
	DisconnectOk       bool
	HandleLocal        bool
	TestLossPacket     func(data []byte) bool
}

//根据选项参数创建一个链路层的endpoint 并返回endpoint的id
func New(opt *Options) tcpip.LinkEndpointID {
	//将该描述符设置为非阻塞  fctnl
	syscall.SetNonblock(opt.FD, true)

	caps := stack.LinkEndpointCapabilities(0)
	if opt.ResolutionRequired {
		caps |= stack.CapabilityResolutionRequired
	}
	if opt.ChecksumOffload {
		caps |= stack.CapabilityChecksumOffload
	}
	if opt.SaveRestore {
		caps |= stack.CapabilitySaveRestore
	}
	if opt.DisconnectOk {
		caps |= stack.CapabilityDisconnectOk
	}

	e := &endpoint{
		fd:          opt.FD,
		mtu:         opt.MTU,
		caps:        caps,
		closed:      opt.CloseFunc,
		addr:        opt.Address,
		hdrSize:     header.EtheernetMinimumsize,
		views:       make([]buffer.View, len(BufConfig)),
		iovecs:      make([]syscall.Iovec, len(BufConfig)),
		handleLocal: opt.HandleLocal,
	}
	log.Println("注册链路层设备，  new endpoint")
	//全局注册链路层设备
	return stack.RegisterLinkEndpoint(e)
}

//返回当前mtu值
func (e *endpoint) MTU() uint32 {
	return e.mtu
}

//返回当前网卡的能力
func (e *endpoint) Capabilities() stack.LinkEndpointCapabilities {
	return e.caps
}

//返回当前以太网头部信息长度
func (e *endpoint) MaxHeaderLength() uint16 {
	return uint16(e.hdrSize)
}

//返回当前MAC地址
func (e *endpoint) LinkAddress() tcpip.LinkAddress {
	return e.addr
}

//将上层的报文经过链路层封装，写入网卡中，如果写入失败则丢弃该报文
func (e *endpoint) WritePacket(r *stack.Route, hdr buffer.Prependable, payload buffer.VectorisedView, protocol tcpip.NetworkProtocolNumber) *tcpip.Error {
	//如果目标地址就是设备本身自己，则将报文重新返回给协议栈
	if e.handleLocal && r.LocalAddress != "" && r.LocalAddress == r.RemoteAddress {
		views := make([]buffer.View, 1, 1+len(payload.Views()))
		views[0] = hdr.View()
		views = append(views, payload.Views()...)
		vv := buffer.NewVectorisedView(len(views[0])+payload.Size(), views)
		e.dispatcher.DeliverNetworkPacket(e, r.RemoteLinkAddress, r.LocalLinkAddress, protocol, vv)
		return nil
	}
	//封装增加以太网头部
	eth := header.Ethernet(hdr.Prepend(header.EtheernetMinimumsize))
	ethHdr := &header.EthernetFields{
		DstAddr: r.RemoteLinkAddress,
		Type:    protocol,
	}

	//如果路由信息中有配置源mac地址，则使用，否则使用本网卡地址
	if r.LocalAddress != "" {
		ethHdr.SrcAddr = r.LocalLinkAddress
	} else {
		ethHdr.SrcAddr = e.addr
	}
	eth.Encode(ethHdr)

	//写入网卡中
	if payload.Size() == 0 {
		return rawfile.NonBlockingWrite(e.fd, hdr.View())
	}
	return rawfile.NonBlockingWrite2(e.fd, hdr.View(), payload.ToView())
}

func (e *endpoint) capViews(n int, buffers []int) int {
	c := 0
	for i, s := range buffers {
		c += s
		if c >= n {
			e.views[i].CapLength(s - (c - n))
			return i + 1
		}
	}
	return len(buffers)
}

//按bufconfig的长度分配内存大小  注意 e.views 和 e.iovecs 共用相同的内存块
func (e *endpoint) allocateViews(bufConfig []int) {
	for i, v := range e.views {
		if v != nil {
			break
		}
		b := buffer.NewView(bufConfig[i])
		e.views[i] = b
		e.iovecs[i] = syscall.Iovec{
			Base: &b[0],
			Len:  uint64(len(b)),
		}
	}
}

//启动从文件描述符中读取数据包的goroutine 协程，并通过提供的分发函数来分发数据包
func (e *endpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.dispatcher = dispatcher

	//链接端点不可靠，保存传输端点后，它们将提至发送传出数据包，并拒绝所有传入数据包
	go e.dispatchLoop()
}

//判断是否Attach了
func (e *endpoint) IsAttached() bool {
	return e.dispatcher != nil
}

//循环从fd读取数据，然后将数据包分发给协议栈
func (e *endpoint) dispatchLoop() *tcpip.Error {
	log.Println("@dispatch 调度进行事件循环接受物理网卡数据 dispatchLoop")
	for {
		cont, err := e.dispatch()
		if err != nil || !cont {
			if e.closed != nil {
				e.closed(err)
			}
			return err
		}
	}
}

//从网卡中读取一个数据报
func (e *endpoint) dispatch() (bool, *tcpip.Error) {
	//读取数据缓存的分配
	e.allocateViews(BufConfig)
	//从网卡中读取数据
	n, err := rawfile.BlockingReadv(e.fd, e.iovecs)
	log.Printf("@step1 物理网卡接受数据read %d bytes", n)
	if err != nil {
		return false, err
	}
	//如果比头部长度还小，直接丢弃
	if n <= e.hdrSize {
		log.Printf("read %d bytes < header bytest %d,比头部长度还小直接丢弃", n, e.hdrSize)
		return false, nil
	}
	var (
		p                             tcpip.NetworkProtocolNumber
		remoteLinkAddr, localLinkAddr tcpip.LinkAddress
	)
	//获取以太网头部信息
	eth := header.Ethernet(e.views[0])
	p = eth.Type()
	remoteLinkAddr = eth.SourceAddress()
	localLinkAddr = eth.DestinationAddress()
	log.Println("@step2 解析以太网协议:", eth, p, remoteLinkAddr, localLinkAddr)

	used := e.capViews(n, BufConfig)
	vv := buffer.NewVectorisedView(n, e.views[:used])
	//将数据内容删除以太网头部信息，也就是将数据指针指向网络层第一个字节
	vv.TrimFront(e.hdrSize)
	//调用nic.delivernetworkpacket来分发网络层数据
	e.dispatcher.DeliverNetworkPacket(e, remoteLinkAddr, localLinkAddr, p, vv)

	for i := 0; i < used; i++ {
		e.views[i] = nil
	}
	return true, nil
}
