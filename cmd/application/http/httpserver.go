package main

import (
	"github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/protocol/application/http"
)

func init() {
	logging.Setup()
}
func main() {
	serv := http.NewHTTP("tap1", "192.168.1.0/24", "192.168.1.1", "8888")
	serv.ListenAndServ()
}
