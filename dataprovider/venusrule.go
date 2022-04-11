package dataprovider

import "fmt"

const (
	VenusRuleRmSource       = 0
	VenusRuleRmSourceNot    = 1
	VenusRuleFileBackup     = 1
	VenusRuleFileBackupNot  = 0
	VenusRuleFileSymlink    = 1
	VenusRuleFileSymlinkNot = 0
)

const (
	VenusRuleModeLocal2Mix    = iota //本地到本地
	VenusRuleModeRemote2Local        //本地到远端
)

type BaseVenusRule struct {
	//ID 表id，不是唯一的id
	ID int64 `json:"id"`
	//Code 规则编码
	Code string `json:"code"`
	// Mode rule
	Pattern string `json:"pattern"`
	// Backup 是否备份0 不备份 1备份
	Backup int `json:"backup"`
	// Mode 0 本地流转，1远端上传，2远端下载
	Mode int `json:"mode"`
	// RmSource 0 不删除，1 删除
	RmSource int `json:"rm_source"`
	// Symlink 0不开启，1 开启 流转文件为创建软连接方式，若开启此选项，必须强制备份文件
	Symlink int `json:"symlink"`
	// AdditionalInfo 附加信息
	AdditionalInfo string `json:"additional_info,omitempty"`
	// CreatedBy 创建人
	CreatedBy string `json:"created_by"`
	// CreatedAt 创建时间，时间戳
	CreatedAt int64 `json:"created_at"`
	// UpdatedBy 更新人
	UpdatedBy string `json:"updated_by"`
	// UpdatedAt 创建时间，时间戳
	UpdatedAt int64 `json:"updated_at"`
}

type VenusRuleOSN struct {
	ID int64 `json:"id"`
	//Username 用户名
	LocalUser string `json:"local_user"`
	//RemoteUser 远端用户
	//RemoteUser string `json:"remote_user"`
	RuleCode string `json:"rule_code"`
	//RelativePath 相对路径
	RelativePath string `json:"relative_path"`
	//AbsolutePath OSS绝对路径
	//OssPath string `json:"absolute_path"`
}

func (o *VenusRuleOSN) getRelativePath() string {
	return o.getRelativePath()
}
func (o *VenusRuleOSN) name() {
}

type VenusRuleHSN struct {
	ID int64 `json:"id"`
	//Username 用户名
	LocalUser string `json:"local_user"`
	//RemoteUser 远端用户
	//RemoteUser string `json:"remote_user"`
	RuleCode string `json:"rule_code"`
	//RelativePath 相对路径
	RelativePath string `json:"relative_path"`
	//AbsolutePath OSS绝对路径
	//OssPath string `json:"absolute_path"`
}

type VenusRule struct {
	BaseVenusRule
	OSN *VenusRuleOSN  `json:"osn"`
	HSN []VenusRuleHSN `json:"hsn"`
}

// GetVenusModeName 获取流转类型
func (vr *VenusRule) GetVenusModeName() string {
	switch vr.Mode {
	case VenusRuleModeRemote2Local:
		return "remote2local"
	case VenusRuleModeLocal2Mix:
		return "local2mix"
	default:
		return "unKnow"
	}
}

// GetLocalUser 获取osn的本地用户名称
func (o *VenusRuleOSN) GetLocalUser() (*User, error) {
	if o.LocalUser == "" {
		return nil, fmt.Errorf("本地用户名为空，无法查找本地用户")
	}
	user, err := UserExists(o.LocalUser)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (o *VenusRuleHSN) GetLocalUser() (*User, error) {
	if o.LocalUser == "" {
		return nil, fmt.Errorf("本地用户名为空，无法查找本地用户")
	}
	user, err := UserExists(o.LocalUser)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
