package http

//HTTP头部，包含若干个键值对，键值对的数量和头部长度
type http_headers struct {
	//HTTP头部中使用的键值对
	ptr  []map[string]string
	len  int
	size int
}

func newHeaders() *http_headers {
	var h *http_headers
	return h
}
//添加新的key-value对到HTTP头部
func (h *http_headers)http_headers_add(key,value string){

    h.ptr[h.len] = map[string]string{
		key:value,
	};
    h.len++;
}
