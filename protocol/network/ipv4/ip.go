package ipv4

import (
	"net"
	"strings"
)

// InternalInterfaces return internal ip && nic name.
func InternalInterfaces() (ip, nic string) {
	inters, err := net.Interfaces()
	if err != nil {
		return "", ""
	}
	for _, inter := range inters {
		if !strings.HasPrefix(inter.Name, "lo") {
			addrs, err := inter.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						return ipnet.IP.String(), inter.Name
					}
				}
			}
		}
	}
	return "", ""
}
