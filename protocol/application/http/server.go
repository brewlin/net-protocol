package http

import (
	"github.com/brewlin/net-protocol/config"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/brewlin/net-protocol/protocol/link/fdbased"
	"github.com/brewlin/net-protocol/protocol/link/tuntap"
	"github.com/brewlin/net-protocol/protocol/transport/udp"

	"github.com/brewlin/net-protocol/pkg/waiter"
	"github.com/brewlin/net-protocol/protocol/network/ipv6"

	"github.com/brewlin/net-protocol/protocol/network/arp"
	"github.com/brewlin/net-protocol/protocol/network/ipv4"
	"github.com/brewlin/net-protocol/protocol/transport/tcp"
	"github.com/brewlin/net-protocol/stack"

	tcpip "github.com/brewlin/net-protocol/protocol"
)


//Server Http
type Server struct {
	socket tcpip.Endpoint
	port   int
	addr   tcpip.Address
	s      *stack.Stack
}

//NewHTTP usage:", os.Args[0], "<tap-device> <local-address/mask> <ipv4-address> <port>
func NewHTTP(tapName, cidrName, addrName, portName string) *Server {
	var server Server
	log.Printf("@application listen:tap :%v addr :%v port :%v", tapName, addrName, portName)
	//解析mac地址
	maddr, err := net.ParseMAC(*config.Mac)
	if err != nil {
		log.Fatal(*config.Mac)
	}
	parseAddr := net.ParseIP(addrName)
	//解析IP地址，ipv4,或者ipv6
	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber
	if parseAddr.To4() != nil {
		addr = tcpip.Address(net.ParseIP(addrName).To4())
		proto = ipv4.ProtocolNumber
	} else if parseAddr.To16() != nil {
		addr = tcpip.Address(net.ParseIP(addrName).To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatal("unkonw iptype")
	}
	localPort, err := strconv.Atoi(portName)
	if err != nil {
		log.Fatalf("unable to convert port")
	}
	if stack.Pstack != nil {
		server.s = stack.Pstack
		server.port = localPort
		server.addr = addr
		return &server
	}

	//虚拟网卡配置
	conf := &tuntap.Config{
		Name: tapName,
		Mode: tuntap.TAP,
	}

	var fd int
	//新建虚拟网卡
	fd, err = tuntap.NewNetDev(conf)
	if err != nil {
		log.Fatal(err)
	}
	//启动网卡
	tuntap.SetLinkUp(tapName)
	//设置路由
	tuntap.SetRoute(tapName, cidrName)

	//抽象网卡层接口
	linkID := fdbased.New(&fdbased.Options{
		FD:                 fd,
		MTU:                1500,
		Address:            tcpip.LinkAddress(maddr),
		ResolutionRequired: true,
	})
	//新建相关协议的协议栈
	s := stack.New([]string{ipv4.ProtocolName, arp.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName}, stack.Options{})
	//新建抽象网卡
	if err := s.CreateNamedNIC(1, "vnic1", linkID); err != nil {
		log.Fatal(err)
	}

	//在该协议栈上添加和注册相关的网络层协议
	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal(err)
	}

	//在该协议栈上添加和注册arp协议
	if err := s.AddAddress(1, arp.ProtocolNumber, arp.ProtocolAddress); err != nil {
		log.Fatal(err)
	}
	//添加默认路由
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: tcpip.Address(strings.Repeat("\x00", len(addr))),
			Mask:        tcpip.AddressMask(strings.Repeat("\x00", len(addr))),
			Gateway:     "",
			NIC:         1,
		},
	})
	server.s = s
	server.port = localPort
	server.addr = addr
	return &server
}

func (s *Server) ListenAndServ() {
	var wq waiter.Queue
	//新建一个tcp端
	ep, e := s.s.NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if e != nil {
		log.Fatal(e)
	}
	//绑定本地端口
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(s.port)}, nil); err != nil {
		log.Fatal("@application http:Bind failed: ", err)
	}
	//监听tcp
	if err := ep.Listen(10); err != nil {
		log.Fatal("@application http:Listen failed: ", err)
	}
	//等待连接 出现
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		n, q, err := ep.Accept()
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				log.Println("@application http:", " now waiting to new client connection ...")
				<-notifyCh
				continue
			}
			log.Println("@application http: Accept() failed: ", err)
			panic(err)
		}

		go s.dispatch(n, q)
	}
}

func (s *Server) dispatch(e tcpip.Endpoint, wq *waiter.Queue) {
	log.Println("@application http: dispatch  got new request")
	fd := NewServerSocket(e, wq)
	con := NewCon(fd)
	con.handler()
	log.Println("@application http: dispatch  close this request")
	con.socket.Close()
}
