package http

import (
	"log"
	"strings"
)

// HTTP请求结构体，包含HTTP方法，版本，URI，HTTP头，内容长
type Request struct {
	//http 请求方法
	method http_method
	//http 版本
	version http_version
	//原始方法
	method_raw string
	//原始版本
	version_raw    string
	uri            string
	port 			int
	headers        *http_headers
	content_length int
	//数据包 body
	body string
}

//初始化一个httprequest
func newRequest() *Request {
	var req Request
	req.content_length = 0
	req.version = HTTP_VERSION_UNKNOWN
	req.content_length = -1
	req.headers = newHeaders()
	return &req

}

//解析httprequest
func (req *Request) parse(con *Connection) {
	buf := con.recv_buf

	req.method_raw, buf = match_until(buf, " ")
	log.Println("@application http: header parse method_raw:", req.method_raw)

	if req.method_raw == "" {
		con.status_code = 400
		return
	}

	// 获得HTTP方法
	req.method = get_method(req.method_raw)
	log.Println("@application http: header parse method:", req.method)

	if req.method == HTTP_METHOD_NOT_SUPPORTED {
		con.set_status_code(501)
	} else if req.method == HTTP_METHOD_UNKNOWN {
		con.status_code = 400
	}

	// 获得URI
	req.uri, buf = match_until(buf, " ")
	log.Println("@application http: header parse uri:", req.uri)

	if req.uri == "" {
		con.status_code = 400
	}

	/*
	 * 判断访问的资源是否在服务器上
	 *
	 */
	// if (resolve_uri(con.real_path, serv.conf.doc_root, req.uri) == -1) {
	// try_set_status(con, 404);
	// }

	// 如果版本为HTTP_VERSION_09立刻退出
	if req.version == HTTP_VERSION_09 {
		con.set_status_code(200)
		req.version_raw = ""
		return
	}

	// 获得HTTP版本
	req.version_raw, buf = match_until(buf, "\r\n")
	log.Println("@application http: header parse version_raw:", req.version_raw)

	if req.version_raw == "" {
		con.status_code = 400
		return
	}

	// 支持HTTP/1.0或HTTP/1.1
	if strings.EqualFold(req.version_raw, "HTTP/1.0") {
		req.version = HTTP_VERSION_10
	} else if strings.EqualFold(req.version_raw, "HTTP/1.1") {
		req.version = HTTP_VERSION_11
	} else {
		con.set_status_code(400)
	}
	log.Println("@application http: header parse version:", req.version)
	log.Println("@application http: header parse status_code:", con.status_code)
	//if con.status_code > 0 {
	//	return
	//}

	// 解析HTTP请求头部

	p := buf
	key, value,tmp := "", "",""
	for p != "" {
		if key, tmp = match_until(p, ": ");key != "" {
			p = tmp
		}
		if value, tmp = match_until(p, "\r\n");value != "" {
			p = tmp
		}
		if key == "" || value == "" {
			break
		}
		req.headers.http_headers_add(key, value)
	}
	//剩下到就是 body
	req.body = p
}

//GetMethod d
func (req *Request) GetMethod() string {
	return req.method_raw
}

//GetHeader header
func (req *Request) GetHeader(h string) string {
	if _, exist := req.headers.ptr[h]; !exist {
		return ""
	}
	return req.headers.ptr[h]
}
//GetBody get get|post data
func (req *Request) GetBody()string{
	return req.body
}
