package docker

import (
	"fmt"
	"github.com/brewlin/net-protocol/pkg/syscall"
	"os"
)

//SetRootDir
func (c *Container) set_rootdir() {
	syscall.ExitIfRootfsNotFound(c.conf.RootDir)

	if err := syscall.PivotRoot(c.conf.RootDir); err != nil {
		fmt.Printf("Error running pivot_root - %s\n", err)
		os.Exit(1)
	}

}
