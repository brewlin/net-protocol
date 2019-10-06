package http

import (
    "strings"
)

// HTTP请求结构体，包含HTTP方法，版本，URI，HTTP头，内容长
type http_request struct {
	//http 请求方法
	method http_method
	//http 版本
	version http_version
	//原始方法
	method_raw string
	//原始版本
	version_raw    string
	uri            string
	headers        *http_headers
	content_length int
}

//初始化一个httprequest
func newRequest() *http_request {
	var req *http_request
	req.content_length = 0
	req.version = HTTP_VERSION_UNKNOWN
	req.content_length = -1
	req.headers = newHeaders()
	return req

}
//解析httprequest
func (req *http_request)parse(con *connection){
    buf := con.recv_buf

    req.method_raw = match_until(buf, " ")

    if req.method_raw == "" {
        con.status_code = 400
        return;
    }

    // 获得HTTP方法
    req.method = get_method(req.method_raw)

    if req.method == HTTP_METHOD_NOT_SUPPORTED {
        con.set_status_code(501)
    } else if(req.method == HTTP_METHOD_UNKNOWN) {
        con.status_code = 400
        return;
    }

    // 获得URI
    req.uri = match_until(buf, " \r\n")

    if req.uri == "" {
        con.status_code = 400
        return
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
    req.version_raw = match_until(buf, "\r\n")

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

    if con.status_code > 0 {
        return
    }

    // 解析HTTP请求头部

    p := buf
    endp := con.recv_buf + con.request_len

    for p < endp {
        key := match_until(p, ": ")
        value := match_until(p, "\r\n")

        if !key || !value {
            con.status_code = 400;
            return
        }
        req.headers.http_headers_add(key,value)
    }

    con.status_code = 200
}
