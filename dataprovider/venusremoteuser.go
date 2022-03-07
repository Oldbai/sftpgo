package dataprovider

import (
	"fmt"
	"github.com/drakkan/sftpgo/v2/util"
)

const (
	venusRemoteUserName = "venus_remote_user"
	remoteProtocolSftp  = 0
	remoteProtocolFtp   = 1
)

type VenusRemoteUser struct {
	//ID 表id，不是唯一的id
	ID int64 `json:"id"`
	//Username 用户名
	Username string `json:"username"`
	//Ip 远端ip地址
	Endpoint string `json:"endpoint"`
	//Protocol '协议：0 sftp，1 ftp',
	Protocol int `json:"protocol"`
	// CreatedBy 创建人
	CreatedBy string `json:"created_by"`
	// CreatedAt 创建时间，时间戳
	CreatedAt int64 `json:"created_at"`
	// UpdatedBy 更新人
	UpdatedBy string `json:"updated_by"`
	// UpdatedAt 创建时间，时间戳
	UpdatedAt int64 `json:"updated_at"`
}

func (remoteUser *VenusRemoteUser) Name() string {
	return fmt.Sprintf("%v %#v", venusRemoteUserName, remoteUser)
}

func (remoteUser *VenusRemoteUser) GetRemoteIp() string {
	return util.GetIPFromRemoteAddress(remoteUser.Endpoint)
}
