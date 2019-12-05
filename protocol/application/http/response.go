package http

import (
	"log"
	"strconv"
)

// HTTP响应结构体，包含内容长度，内容，HTTP头部
type Response struct {
	content_length int
	entity_body    string
	headers        *http_headers
	con            *Connection
}

func newResponse(con *Connection) *Response {
	var resp Response

	resp.headers = newHeaders()
	resp.entity_body = ""
	resp.content_length = -1
	resp.con = con
	return &resp
}

// 发送HTTP响应
func (r *Response) send() {
	if r.con.request.version == HTTP_VERSION_09 {
		r.send_http09_response()
	} else {
		r.send_response()
	}
}

/*
 * HTTP_VERSION_09时，只发送响应消息内容，不包含头部
 */
func (r *Response) send_http09_response() {

	// 检查文件是否可以访问 && check_file_attrs(con, con.real_path)
	if r.con.status_code == 200 {
		// read_file(r.entity_body, con.real_path);
	} else {
		// read_err_file(serv, con, r.entity_body);
	}

	r.send_all(r.entity_body)
}

/*
 * 构建响应消息和消息头部并发送消息
 * 如果请求的资源无法打开则发送错误消息
 */
func (r *Response) send_response() {
	h := r.headers
	h.http_headers_add("Server", "github.com/brewlin/net-protocol/1.00")
	h.http_headers_add("Connection", "close")
	if r.entity_body == "" {
		r.entity_body = default_success_msg
	}

	if r.con.status_code != 200 {
		r.send_err_response()
		return
	}

	// if (check_file_attrs(con, con->real_path) == -1) {
	// send_err_response(serv, con);
	// return;
	// }

	// if r.method != HTTP_METHOD_HEAD {
	// read_file(resp->entity_body, con->real_path);
	// }

	// 构建消息头部
	// const char *mime = get_mime_type(con->real_path, "text/plain");
	// http_headers_add(resp->headers, "Content-Type", mime);
	// http_headers_add_int(resp->headers, "Content-Length", resp->content_length);

	r.build_and_send_response()
}

//End send the body
func (r *Response) End(buf string) {
	r.entity_body = buf
}

/*
 * 当出错时发送标准错误页面，页面名称类似404.html
 * 如果错误页面不存在则发送标准的错误消息
 */
func (r *Response) send_err_response() {
	// snprintf(err_file, sizeof(err_file), "%s/%d.html", serv->conf->doc_root, con->status_code);

	// 检查错误页面
	// if (check_file_attrs(con, err_file) == -1) {
	// resp->content_length = strlen(default_err_msg);
	// log_error(serv, "failed to open file %s", err_file);
	// }
	r.entity_body = default_err_msg

	// 构建消息头部
	r.headers.http_headers_add("Content-Type", "text/html")
	r.headers.http_headers_add("Content-Length", string(r.content_length))

	// if con.request.method != HTTP_METHOD_HEAD {
	//    read_err_file(serv, con, resp->entity_body);
	// }

	r.build_and_send_response()
}

// 构建并发送响应
func (r *Response) build_and_send_response() {

	// 构建发送的字符串
	buf := ""

	buf += r.con.request.version_raw + " "
	buf += strconv.Itoa(r.con.status_code)
	buf += " "
	buf += StatusText(r.con.status_code)
	buf += "\r\n"
	for k, v := range r.headers.ptr {
		buf += k
		buf += ": "
		buf += v
		buf += "\r\n"
	}
	buf += "\r\n"
	buf += r.entity_body
	log.Println("@application http:response send 构建http响应包体")
	log.Println(buf)
	// 将字符串缓存发送到客户端
	r.send_all(buf)
}

/**
 * 将响应消息发送给客户端
 */
func (r *Response) send_all(buf string) {
	r.con.socket.Write([]byte(buf))
}

//Error set status_code
func (r *Response) Error(code int) {
	r.con.set_status_code(code)
}

//GetCon get
func (r *Response) GetCon() *Connection {
	return r.con
}
