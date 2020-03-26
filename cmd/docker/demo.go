package  main

import "github.com/brewlin/net-protocol/docker"

//$
//$ mkdir -p /tmp/docker/
//$ tar -C /tmp/docker/ -xf assets/busybox.tar
func  main()  {
	conf := docker.ContainerConfig{
		HostName:   "docker",
		RootDir:    "/tmp/docker/",
		Ip:         "",
		BridgeName: "",
		BridgeIp:   "",
	}
	container := docker.NewContainer(&conf)
	container.Start()
}