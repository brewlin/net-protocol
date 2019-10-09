package http

import (
	"fmt"

	"strconv"

	"github.com/brewlin/net-protocol/pkg/buffer"
	tcpip "github.com/brewlin/net-protocol/protocol"
)

// HTTP响应结构体，包含内容长度，内容，HTTP头部
type http_response struct {
	content_length int
	entity_body    string
	headers        *http_headers
}

func newResponse() *http_response {
	var resp http_response

	resp.headers = newHeaders()
	resp.entity_body = ""
	resp.content_length = -1
	return &resp
}

// 发送HTTP响应
func (r *http_response) send(con *connection) {
	if con.request.version == HTTP_VERSION_09 {
		r.send_http09_response(con)
	} else {
		r.send_response(con)
	}
}

/*
 * HTTP_VERSION_09时，只发送响应消息内容，不包含头部
 */
func (r *http_response) send_http09_response(con *connection) {

	// 检查文件是否可以访问 && check_file_attrs(con, con.real_path)
	if con.status_code == 200 {
		// read_file(r.entity_body, con.real_path);
	} else {
		// read_err_file(serv, con, r.entity_body);
	}

	r.send_all(con, r.entity_body)
}

/*
 * 构建响应消息和消息头部并发送消息
 * 如果请求的资源无法打开则发送错误消息
 */
func (r *http_response) send_response(con *connection) {
	h := r.headers
	h.http_headers_add("Server", "github.com/brewlin/net-protocol/1.00")
	h.http_headers_add("Connection", "close")
	r.entity_body = default_success_msg

	if con.status_code != 200 {
		r.send_err_response(con)
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

	r.build_and_send_response(con)
}

/*
 * 当出错时发送标准错误页面，页面名称类似404.html
 * 如果错误页面不存在则发送标准的错误消息
 */
func (r *http_response) send_err_response(con *connection) {
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

	r.build_and_send_response(con)
}

// 构建并发送响应
func (r *http_response) build_and_send_response(con *connection) {

	// 构建发送的字符串
	buf := ""

	buf += con.request.version_raw + " "
	buf += strconv.Itoa(con.status_code)
	buf += " "
	buf += r.reason_phrase(con.status_code)
	buf += "\r\n"
	for k, v := range r.headers.ptr {
		buf += k
		buf += ": "
		buf += v
		buf += "\r\n"
	}
	buf += "\r\n"
	buf += r.entity_body
	fmt.Println("@application http:response send 构建http响应包体")
	fmt.Println(buf)
	// 将字符串缓存发送到客户端
	r.send_all(con, buf)
}

//根据状态码构建响应结构中的状态消息
func (r *http_response) reason_phrase(status_code int) string {
	switch status_code {
	case 200:
		return "ok"
	case 400:
		return "Bad Request"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	case 501:
		return "Not Implemened"
	}
	return ""
}

/**
 * 将响应消息发送给客户端
 */
func (r *http_response) send_all(con *connection, buf string) {
	v := buffer.View(buf)
	con.socket.Write(tcpip.SlicePayload(v), tcpip.WriteOptions{To: con.addr})
}
