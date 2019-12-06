package websocket

import (
	"encoding/binary"
	"errors"
	"log"

	"github.com/brewlin/net-protocol/protocol/application/http"
)

const (
	/*
	 * 是否是最后一个数据帧
	 * Fin Rsv1 Rsv2 Rsv3 Opcode
	 *  1  0    0    0    0 0 0 0  => 128
	 */
	finalBit = 1 << 7

	/*
	 * 是否需要掩码处理
	 *  Mask payload-len 第一位mask表示是否需要进行掩码处理 后面
	 *  7位表示数据包长度 1.0-125 表示长度 2.126 后面需要扩展2 字节 16bit
	 *  3.127则扩展8bit
	 *  1    0 0 0 0 0 0 0  => 128
	 */
	maskBit = 1 << 7

	/*
	 * 文本帧类型
	 * 0 0 0 0 0 0 0 1
	 */
	TextMessage = 1
	/*
	 * 关闭数据帧类型
	 * 0 0 0 0 1 0 0 0
	 */
	CloseMessage = 8
)

//websocket 连接
type Conn struct {
	writeBuf []byte
	maskKey  [4]byte
	conn     *http.Connection
}

func newConn(conn *http.Connection) *Conn {
	return &Conn{conn: conn}
}
func (c *Conn) Close() {
	c.conn.Close()
}

//发送数据
func (c *Conn) SendData(data []byte)error {
	length := len(data)
	c.writeBuf = make([]byte, 10+length)

	//数据开始和结束位置
	payloadStart := 2
	/**
	 *数据帧的第一个字节，不支持且只能发送文本类型数据
	 *finalBit 1 0 0 0 0 0 0 0
	 *                |
	 *Text     0 0 0 0 0 0 0 1
	 * =>      1 0 0 0 0 0 0 1
	 */
	c.writeBuf[0] = byte(TextMessage) | finalBit
	log.Printf("1 bit:%b\n", c.writeBuf[0])

	//数据帧第二个字节，服务器发送的数据不需要进行掩码处理
	switch {
	//大于2字节的长度
	case length >= 1<<16: //65536
		//c.writeBuf[1] = byte(0x00) | 127 // 127
		c.writeBuf[1] = byte(127) // 127
		//大端写入64位
		binary.BigEndian.PutUint64(c.writeBuf[payloadStart:], uint64(length))
		//需要8byte来存储数据长度
		payloadStart += 8
	case length > 125:
		//c.writeBuf[1] = byte(0x00) | 126
		c.writeBuf[1] = byte(126)
		binary.BigEndian.PutUint16(c.writeBuf[payloadStart:], uint16(length))
		payloadStart += 2
	default:
		//c.writeBuf[1] = byte(0x00) | byte(length)
		c.writeBuf[1] = byte(length)
	}
	log.Printf("2 bit:%b\n", c.writeBuf[1])

	copy(c.writeBuf[payloadStart:], data[:])
	return c.conn.Write(c.writeBuf[:payloadStart+length])
}

//读取数据
func (c *Conn) ReadData() (data []byte, err error) {
	var b [8]byte
	//读取数据帧的前两个字节
	if _, err := c.conn.Readn(b[:2]); err != nil {
		return nil, err
	}
	//开始解析第一个字节 是否还有后续数据帧
	final := b[0]&finalBit != 0
	log.Printf("read data 1 bit :%b\n", b[0])
	//不支持数据分片
	if !final {
		log.Println("Recived fragemented frame,not support")
		return nil, errors.New("not suppeort fragmented message")
	}

	//数据帧类型
	/*
	 *1 0 0 0  0 0 0 1
	 *        &
	 *0 0 0 0  1 1 1 1
	 *0 0 0 0  0 0 0 1
	 * => 1 这样就可以直接获取到类型了
	 */
	frameType := int(b[0] & 0xf)
	//如果 关闭类型，则关闭连接
	if frameType == CloseMessage {
		c.conn.Close()
		log.Println("Recived closed message,connection will be closed")
		return nil, errors.New("recived closed message")
	}
	//只实现了文本格式的传输,编码utf-8
	if frameType != TextMessage {
		return nil, errors.New("only support text message")
	}
	//检查数据帧是否被掩码处理
	//maskBit => 1 0 0 0 0 0 0 0 任何与他 要么为0 要么为 128
	mask := b[1]&maskBit != 0
	//数据长度
	payloadLen := int64(b[1] & 0x7F) //0 1 1 1 1 1 1 1 1 127
	dataLen := int64(payloadLen)
	//根据payload length 判断数据的真实长度
	switch payloadLen {
	case 126: //扩展2字节
		if _, err := c.conn.Readn(b[:2]); err != nil {
			return nil, err
		}
		//获取扩展二字节的真实数据长度
		dataLen = int64(binary.BigEndian.Uint16(b[:2]))
	case 127:
		if _, err := c.conn.Readn(b[:8]); err != nil {
			return nil, err
		}
		dataLen = int64(binary.BigEndian.Uint64(b[:8]))
	}

	log.Printf("Read data length :%d,payload length %d", payloadLen, dataLen)
	//读取mask key
	if mask { //如果需要掩码处理的话 需要取出key
		//maskKey 是 4 字节  32位
		if _, err := c.conn.Readn(c.maskKey[:]); err != nil {
			return nil, err
		}
	}
	//读取数据内容
	p := make([]byte, dataLen)
	if _, err := c.conn.Readn(p); err != nil {
		return nil, err
	}
	if mask {
		maskBytes(c.maskKey, p) //进行解码
	}
	return p, nil
}
