# http协议
## start
```
cd application/http
go build
sudo ./http

curl 192.168.1.1:8888/test
```
## @httpserver.go
```
import (
	"fmt"

	"github.com/brewlin/net-protocol/pkg/logging"
	"github.com/brewlin/net-protocol/protocol/application/http"
)

func init() {
	logging.Setup()
}
func main() {
	serv := http.NewHTTP("tap1", "192.168.1.0/24", "192.168.1.1", "9502")
	serv.HandleFunc("/", func(request *http.Request, response *http.Response) {
		fmt.Println("hell0 ----------------------")
		response.End("hello")
	})
	serv.ListenAndServ()
}
```
## 浏览器console
![](/resource/http.png)
## @终端log
```
@application http:  now waiting to new client connection ...
@application http: dispatch  got new request
@应用层 http: waiting new event trigger ...
http协议原始数据:
GET /test HTTP/1.1
Host: 192.168.1.1:8888
User-Agent: curl/7.47.0
Accept: */*


@application http: header parse method_raw: GET
@application http: header parse method: 1
@application http: header parse uri: /test
@application http: header parse version_raw: HTTP/1.1
@application http: header parse version: 3
@application http: header parse status_code: 0
@application http:response send 构建http响应包体
HTTP/1.1 200 ok
Server: github.com/brewlin/net-protocol/1.00
Connection: close

<HTML><HEAD><TITLE>SUCCESS</TITLE></HEAD><BODY><H1>github.com/brewlin/net-protocol/http</H1></BODY></HTML>
@application http: dispatch  close this request

```