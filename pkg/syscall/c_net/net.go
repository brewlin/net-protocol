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
	"syscall"
	"unsafe"
)

//LxcBridgeAttach
//bind the veth to bridge
func LxcBridgeAttach(bridge, ifname string) (state int, err error) {
	b := C.CString(bridge)
	defer C.free(unsafe.Pointer(b))

	i := C.CString(ifname)
	defer C.free(unsafe.Pointer(i))
	cstate :=  C.lxc_bridge_attach(b,i)

	state = int(cstate)
	if cstate != 0 {
		err = errors.New(syscall.Errno(cstate).Error())
	}
	return
}

//GetHardwareAddr
func GetHardwareAddr(name string) (string, error) {
	n := C.CString(name)
	defer C.free(unsafe.Pointer((n)))

	res := C.get_hardware_addr(n)
	defer C.free(unsafe.Pointer(res))
	mac := C.GoString(res)
	return mac, nil
}
