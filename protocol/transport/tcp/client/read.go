package client

import (
	"errors"

	tcpip "github.com/brewlin/net-protocol/protocol"
)

//Read 一次性读取完缓冲区数据
func (c *Client) Read() ([]byte, error) {
	<-c.notifyC
	var buf []byte
	var err error
	for {
		v, _, e := c.ep.Read(&c.remote)
		if e != nil {
			err = e
			break
		}
		buf = append(buf, v...)
	}
	if buf == nil {
		return nil, err
	}
	return buf, nil

}

//Readn  读取固定字节的数据
func (c *Client) Readn(p []byte) (int, error) {
	c.bufmu.Lock()
	defer c.bufmu.Unlock()
	//获取足够长度的字节
	if len(p) > len(c.buf) {

		for {
			if len(p) <= len(c.buf) {
				break
			}
			buf, _, err := c.ep.Read(&c.remote)
			if err != nil {
				if err == tcpip.ErrWouldBlock {
					//阻塞等待数据
					<-c.notifyC
					continue
				}
				return 0, err
			}
			c.buf = append(c.buf, buf...)
		}
	}
	if len(p) > len(c.buf) {
		return 0, errors.New("package len is smaller than p need")
	}

	n := copy(p, c.buf)
	c.buf = c.buf[len(p):]
	return n, nil
}
