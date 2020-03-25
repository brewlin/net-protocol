package c_net

import "testing"

func TestLxcBridgeAttach(t *testing.T) {
	code,err := LxcBridgeAttach("docker0","tap0")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(code)
}

func TestGetHardwareAddr(t *testing.T) {
	res,err := GetHardwareAddr("tap0")
	if err != nil {
		t.Log(err)
		t.Fatal(err)
	}
	if len(res) != 6 {
		t.Fatal(err)
	}
	t.Log(res)
}