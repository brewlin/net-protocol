package c_net

/*
#include <stdlib.h>
#include <errno.h>
#include <string.h>
#include "network.h"
*/
import "C"
import (
	"errors"
	"unsafe"
)




//LxcBridgeAttach
//bind the veth to bridge
func LxcBridgeAttach(bridge , ifname string) (int,error){
	b := C.CString(bridge)
	defer C.free(unsafe.Pointer(b))

	i := C.CString(ifname)
	defer C.free(unsafe.Pointer(i))
	err :=  C.lxc_bridge_attach(b,i)
	if int(err) != 0 {
		return int(err),errors.New(C.GoString(C.strerror(err)))
	}
	return int(err) ,nil
}

//GetHardwareAddr
func GetHardwareAddr(name string) (string,error){
	n := C.CString(name)
	defer C.free(unsafe.Pointer((n)))

	res := C.get_hardware_addr(n)
	defer C.free(unsafe.Pointer(res))
	mac := C.GoString(res)
	return mac,nil
}