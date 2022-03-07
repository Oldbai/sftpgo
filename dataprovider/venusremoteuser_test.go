package dataprovider

import (
	"fmt"
	"testing"
)

func TestName(t *testing.T) {
	r := &VenusRemoteUser{Endpoint: "127.0.0.1:1080"}
	fmt.Println(r.Name())
}

func TestVenusRemoteUser_GetRemoteIp(t *testing.T) {
	r := &VenusRemoteUser{Endpoint: "127.0.0.1:1080"}
	fmt.Println(r.GetRemoteIp())

	r1 := VenusRemoteUser{Endpoint: ""}
	fmt.Println(r1.GetRemoteIp())
}
