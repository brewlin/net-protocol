package main

import (
	"io/ioutil"
	"log"
	"net"
)

func main() {
	addr := "192.168.1.1:9111"
	tcpaddr, err := net.ResolveTCPAddr("", addr)
	if err != nil {
		log.Fatal("net Resolvetcp addr error!", err.Error())
	}
	log.Println("str tcpaddr = ", tcpaddr.String())
	log.Println("str network = ", tcpaddr.Network())

	conn, err := net.DialTCP("tcp4", nil, tcpaddr)
	log.Println("dial over")
	if err != nil {
		log.Fatal("net diatcp error!", err.Error())
	}
	defer conn.Close()

	blen, err := conn.Write([]byte("HEAD / HTTP/1.0 \r\n\r\n"))
	if err != nil {
		log.Fatal("err = ", err.Error())
	}
	log.Println("blen = ", blen)
	res, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Println("res = ", string(res))
	log.Println(conn.LocalAddr())
	log.Println(conn.RemoteAddr())

}
