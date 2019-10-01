package http

// HTTP响应结构体，包含内容长度，内容，HTTP头部
type http_response struct {
	content_length int
	entity_body    string
	headers        *http_headers
}

func newResponse() *http_response {
	var resp *http_response

	resp.headers = newHeaders()
	resp.entity_body = ""
	resp.content_length = -1
	return resp
}
