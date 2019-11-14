package tuntap

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	TUN = 1
	TAP = 2
)

var (
	ErrDeviceMode = errors.New("unsupport device mode")
)

type rawSockaddr struct {
	Family uint16
	Data   [14]byte
}

//创建虚拟网卡
type Config struct {
	Name string //网卡名
	Mode int    //网卡模式 TUN OR TAP
}

//NewNetDev 根据配置返回虚拟网卡的文件描述符
func NewNetDev(c *Config) (fd int, err error) {
	switch c.Mode {
	case TUN:
		fd, err = newTun(c.Name)
	case TAP:
		fd, err = newTap(c.Name)
	default:
		err = ErrDeviceMode
		return
	}
	if err != nil {
		return
	}
	return
}

//CreateTap 通过命令行 ip 创建网卡
func CreateTap(name string) (err error) {
	//ip tuntap add mode tap tap0
	out, cmdErr := exec.Command("ip", "tuntap", "add", "mode", "tap", name).CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	return
}

//SetLinkUp 让系统启动该网卡
func SetLinkUp(name string) (err error) {
	//ip link set <device_name> up
	out, cmdErr := exec.Command("ip", "link", "set", name, "up").CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	return
}

//SetRoute 通过ip命令添加路由
func SetRoute(name, cidr string) (err error) {
	//ip route add 192.168.1.0/24 dev tap0
	out, cmdErr := exec.Command("ip", "route", "add", cidr, "dev", name).CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	return
}

//AddIP 通过ip命令添加ip地址
func AddIP(name, ip string) (err error) {
	//ip addr add 192.168.1.1 dev tap0
	out, cmdErr := exec.Command("ip", "addr", "add", ip, "dev", name).CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	return
}

//AddGateWay 通过ip命令 添加网关
func AddGateWay(dst, gateway, name string) (err error) {
	//ip route add default via gateway dev tap
	out, cmdErr := exec.Command("ip", "route", "add", dst, "via", gateway, "dev", name).CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	return
}

//DelTap 删除网卡
func DelTap(name string) (err error) {
	out, cmdErr := exec.Command("ip", "tuntap", "del", "mode", "tap", name).CombinedOutput()
	if cmdErr != nil {
		err = fmt.Errorf("%v:%v", cmdErr, string(out))
		return
	}
	return
}

//GetHardwareAddr gethard
func GetHardwareAddr(name string) (string, error) {
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return "", err
	}
	defer syscall.Close(fd)

	var ifreq struct {
		name [16]byte
		addr rawSockaddr
		_    [8]byte
	}
	copy(ifreq.name[:], name)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCGIFHWADDR, uintptr(unsafe.Pointer(&ifreq)))
	if errno != 0 {
		return "", errno
	}
	mac := ifreq.addr.Data[:6]
	return string(mac[:]), nil
}

//newTun 新建一个tun模式的虚拟网卡，然后返回该网卡的文件描述符 IFF_NO_PI表示不需要包信息
func newTun(name string) (int, error) {
	return open(name, syscall.IFF_TUN|syscall.IFF_NO_PI)
}

//newTAP 新建一个tap模式的虚拟网卡，然后返回该网卡的文件描述符
func newTap(name string) (int, error) {
	return open(name, syscall.IFF_TAP|syscall.IFF_NO_PI)
}

//先打开一个字符串设备，通过系统调用将虚拟网卡和字符串设备fd绑定在一起
func open(name string, flags uint16) (int, error) {
	//打开tuntap的字符设备，得到字符设备的文件描述符
	fd, err := syscall.Open("/dev/net/tun", syscall.O_RDWR, 0)
	if err != nil {
		return -1, err
	}

	var ifr struct {
		name  [16]byte
		flags uint16
		_     [22]byte
	}
	copy(ifr.name[:], name)
	ifr.flags = flags
	//通过ioctl系统调用，将fd和虚拟网卡驱动绑定在一起
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TUNSETIFF, uintptr(unsafe.Pointer(&ifr)))
	if errno != 0 {
		syscall.Close(fd)
		return -1, errno
	}
	return fd, nil
}
