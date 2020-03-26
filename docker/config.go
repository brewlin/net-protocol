package docker

//ContainerConfig
type ContainerConfig struct {
	//container host name
	HostName string
	//image path
	RootDir string

	//container ip addr
	Ip string

	//bridge name & ip
	BridgeName string
	BridgeIp string
}
