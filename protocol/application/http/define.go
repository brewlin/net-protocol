package http

// HTTP请求的方
type http_method int

const (
	//不识别该 未知方法
	HTTP_METHOD_UNKNOWN http_method = -1
	//不支持的请求方法
	HTTP_METHOD_NOT_SUPPORTED http_method = 0
	//GET请求方法
	HTTP_METHOD_GET http_method = 1
	//HEAD请求方法
	HTTP_METHOD_HEAD http_method = 2
)

//http 版本
type http_version int

const (
	HTTP_VERSION_UNKNOWN http_version = iota
	HTTP_VERSION_09
	HTTP_VERSION_10
	HTTP_VERSION_11
)

//接受状态
type http_recv_state int

const (
	HTTP_RECV_STATE_WORD1 http_recv_state = iota
	HTTP_RECV_STATE_WORD2
	HTTP_RECV_STATE_WORD3
	HTTP_RECV_STATE_SP1
	HTTP_RECV_STATE_SP2
	HTTP_RECV_STATE_LF
	HTTP_RECV_STATE_LINE
)

const default_err_msg = "<HTML><HEAD><TITLE>ERROR</TITLE></HEAD><BODY><H1>SOMETING WRONG</H1></BODY></HTML>"
const default_success_msg = "<HTML><HEAD><TITLE>SUCCESS</TITLE></HEAD><BODY><H1>github.com/brewlin/net-protocol/http</H1></BODY></HTML>"

var mime = map[string]string{
	".html": "text/html",
	".css":  "text/css",
	".js":   "application/javascript",
	".jpg":  "image/jpg",
	".png":  "image/png",
}
