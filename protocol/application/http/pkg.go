package http

import (
	"strings"
)

// 一直读取字符直到遇到delims字符串，将先前读取的返回
// 可以用在解析HTTP请求时获取HTTP方法名（第一段字符串
func match_until(buf,delims string)string{
	i := strings.Index(buf,delims)
	return buf[:i]
}
// 根据字符串获得请求中的HTTP方法，目前只支持GET和HEAD
func get_method(method string)http_method{
	if strings.EqualFold(method, "GET"){
		return HTTP_METHOD_GET;
	}else if strings.EqualFold(method, "HEAD") {
		return HTTP_METHOD_HEAD;
	}else if strings.EqualFold(method,"POST") || strings.EqualFold(method,"PUT") {
		return HTTP_METHOD_NOT_SUPPORTED;
	}
	return HTTP_METHOD_UNKNOWN
}