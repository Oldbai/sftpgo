package dataprovider

const (
	modeLocal      = 0
	modeRemoteUp   = 1
	moreRemoteDown = 2
	rmSource       = 0
	rmSourceNot    = 1
	fileBackup     = 1
	fileBackupNot  = 0
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
	LocalUser []User `json:"local_user"`
	//RemoteUser 远端用户
	RemoteUser []VenusRemoteUser `json:"remote_user"`
	//RelativePath 相对路径
	RelativePath string `json:"relative_path"`
	//AbsolutePath 绝对路径
	AbsolutePath string `json:"absolute_path"`
}

type VenusRuleHSN struct {
	VenusRuleOSN
}

type VenusRule struct {
	BaseVenusRule
	OSN []VenusRuleOSN `json:"osn"`
	HSN []VenusRuleHSN `json:"hsn"`
}
