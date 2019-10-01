package http

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
