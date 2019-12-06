package websocket

import (
	"errors"
	"fmt"
	"log"

	"github.com/brewlin/net-protocol/protocol/application/http"
)

func Upgrade(r *http.Request, w *http.Response) (c *Conn, err error) {
	//是否是Get方法
	if r.GetMethod() != "GET" {
		w.Error(http.StatusMethodNotAllowed)
		return nil, errors.New("websocket:method not GET")
	}
	//检查 Sec-WebSocket-Version 版本
	if values := r.GetHeader("Sec-WebSocket-Version"); values != "13" {
		w.Error(http.StatusBadRequest)
		return nil, errors.New("websocket:version != 13")
	}

	//检查Connection 和  Upgrade
	if values := r.GetHeader("Connection"); !tokenListContainsValue(values, "upgrade") {
		w.Error(http.StatusBadRequest)
		return nil, errors.New("websocket:could not find connection header with token 'upgrade'")
	}
	if values := r.GetHeader("Upgrade"); values != "websocket" {
		w.Error(http.StatusBadRequest)
		return nil, errors.New("websocket:could not find connection header with token 'websocket'")
	}

	//计算Sec-Websocket-Accept的值
	challengeKey := r.GetHeader("Sec-WebSocket-Key")
	if challengeKey == "" {
		w.Error(http.StatusBadRequest)
		return nil, errors.New("websocket:key missing or blank")
	}

	//接管当前tcp连接，阻止内置http接管连接
	con := w.GetCon()
	// 构造握手成功后返回的 response
	p := []byte{}
	p = append(p, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: "...)
	p = append(p, computeAcceptKey(challengeKey)...)
	p = append(p, "\r\n\r\n"...)
	//返回repson 但不关闭连接
	if err = con.Write(p); err != nil {
		fmt.Println(err)
		fmt.Println("write p err", err)
		return nil, err
	}
	//升级为websocket
	log.Println("Upgrade http to websocket successfully")
	conn := newConn(con)
	return conn, nil
}
