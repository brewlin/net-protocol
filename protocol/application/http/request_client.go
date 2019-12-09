package http

import (
	"log"
	"strconv"
)

func (r *Request)init(path,ip string,port int)  {
	r.uri = path
	r.port = port
	r.headers.ptr = map[string]string{
		"Host": ip+":" + strconv.Itoa(port),
		"User-Agent": "net-protocol/5.0",
		"Accept":"*/*",
	}
}
func (r *Request) send()string{
	if r.method_raw == "" {
		r.method_raw = "GET"
	}
	// 构建发送的字符串
	buf := ""
	buf += r.method_raw + " "
	buf += r.uri + " "
	buf += "HTTP/1.1"+ "\r\n"
	for k, v := range r.headers.ptr {
		buf += k
		buf += ": "
		buf += v
		buf += "\r\n"
	}
	buf += "\r\n"
	buf += r.body
	log.Println("@application http:request send 构建http请求包体")
	log.Println(buf)
	return buf
}