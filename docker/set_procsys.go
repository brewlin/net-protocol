package docker

import (
	"fmt"
	"github.com/brewlin/net-protocol/pkg/syscall"
	"os"
)

//SetProcSys
func (c *Container) set_procsys() {
	newrootPath := os.Args[1]
	if err := syscall.MountProc(newrootPath); err != nil {
		fmt.Printf("Error mounting /proc - %s\n", err)
		os.Exit(1)
	}
	if err := syscall.MountSys(newrootPath); err != nil {
		fmt.Printf("Error mounting /sys - %s\n", err)
		os.Exit(1)
	}

}