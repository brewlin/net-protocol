package buffer

import (
	"errors"
	"log"
	"regexp"
	"strconv"
)
//ResolveUrl parse url to ip port path
//http://10.0.2.15/test
//http://10.0.2.15:8080/
func ParseUrl(url string)(ip string,port int,path string,err error){
	// ipex := regexp.MustCompile(`http://(.*?)(:(\d{2,4}))?(/.*)`)
	egex := regexp.MustCompile(`http://(\d+\.\d+\.\d+\.\d+)(:(\d{2,4}))?(/.*)`)
	regex := egex.FindAllStringSubmatch(url,-1)
	if len(regex) != 1 {
		return ip,port,path,errors.New("url is invalid")
	}
	urlPar := regex[0]
	var oport string
	ip,oport,path = urlPar[1],urlPar[3],urlPar[4]
	log.Println(urlPar)
	if oport == "" {
		oport = "80"
	}
	port,err = strconv.Atoi(oport)   
	return
}