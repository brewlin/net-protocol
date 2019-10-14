package websocket

import (
	"crypto/sha1"
	"encoding/base64"
	"strings"
)

var KeyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

//握手阶段使用 加密key返回 进行握手
func computeAcceptKey(challengeKey string) string {
	h := sha1.New()
	h.Write([]byte(challengeKey))
	h.Write(KeyGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

//解码
func maskBytes(key [4]byte, b []byte) {
	pos := 0
	for i := range b {
		b[i] ^= key[pos&3]
		pos++
	}
}

// 检查http 头部字段中是否包含指定的值
func tokenListContainsValue(h string, value string) bool {
	for _, s := range strings.Split(h, ",") {
		if strings.EqualFold(value, strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}
