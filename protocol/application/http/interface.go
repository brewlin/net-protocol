package http

//Socket 提供数据读写io接口
type Socket interface{
	//Write write
	Write(buf []byte) error 
	//Read data
	Read() ([]byte, error)
	//Readn  读取固定字节的数据
	Readn(p []byte) (int, error)
	Close()

}