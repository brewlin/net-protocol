package http

import (
	"sync"
)

type ServeMux struct {
	mu sync.RWMutex
	m  map[string]muxEntry
}

type muxEntry struct {
	h       func(*Request, *Response)
	pattern string
}

var defaultMux ServeMux

//handle
func (mu *ServeMux) dispatch(con *Connection) {
	if _, exist := defaultMux.m[con.request.uri]; !exist {
		con.set_status_code(400)
		return
	}
	defaultMux.m[con.request.uri].h(con.request, con.response)
}

//HandleFunc handle pattern
func (s *Server) HandleFunc(pattern string, handler func(*Request, *Response)) {
	defaultMux.mu.Lock()
	defer defaultMux.mu.Unlock()

	if pattern == "" {
		panic("invalid pattern url")
	}
	if handler == nil {
		panic("http: nil handler")
	}
	if _, exist := defaultMux.m[pattern]; exist {
		panic("http: multiple registrations for " + pattern)
	}

	if defaultMux.m == nil {
		defaultMux.m = make(map[string]muxEntry)
	}
	e := muxEntry{h: handler, pattern: pattern}
	defaultMux.m[pattern] = e
}
