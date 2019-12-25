package udp

import (
	"flag"
	"log"
	"net"
)

func main() {
	var (
		addr = flag.String("a", "192.168.1.1:9000", "udp dst address")
	)
	log.SetFlags(log.Lshortfile)

	udpAddr, err := net.ResolveUDPAddr("udp", *addr)
	if err != nil {
		panic(err)
	}
	//建立udp连接
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		panic(err)
	}

	send := []byte("hello")
	recv := make([]byte, 10)
	if _, err := conn.Write(send); err != nil {
		log.Fatal(err)
	}
	log.Printf("send:%s", string(send))

	rn, _, err := conn.ReadFrom(recv)
	log.Println(rn)
	if err != nil {
		panic(err)
	}
	log.Printf("recv :%s", string(recv[:rn]))
}
