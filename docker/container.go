package docker

import (
	"fmt"
	"github.com/brewlin/net-protocol/pkg/reexec"
	"os"
	"syscall"
)

type Container struct {
	conf *ContainerConfig
}
//init
func (c *Container) init() {

	c.set_procsys()
	c.set_rootdir()
	syscall.Sethostname([]byte(c.conf.HostName))
	c.run_bash()

}
func NewContainer(conf *ContainerConfig) *Container {
	return &Container{conf:conf}
}
//Start
func (c *Container) Start() {
	reexec.Register("init",c.init)
	if reexec.Init() {
		fmt.Println("exit")
		os.Exit(0)
	}

	cmd := reexec.Command("init",c.conf.RootDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = []string{"PS1=-[ns-process]- # "}
	fmt.Println(cmd.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNS  |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running the /bin/sh command - %s\n", err)
		os.Exit(1)
	}
}
