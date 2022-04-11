package venus

import (
	"fmt"
	"github.com/drakkan/sftpgo/v2/logger"
	"github.com/drakkan/sftpgo/v2/vfs"
	"github.com/sftpgo/sdk/plugin/notifier"
)

const (
	logSender = "venus"
)

//Msg venus流转任务消息
type Msg struct {
	Action            string `json:"action"`                //用户操作类型
	Username          string `json:"username"`              //用户名
	Path              string `json:"path"`                  //用户路径
	TargetPath        string `json:"target_path,omitempty"` //目标路径，rename或copy后的路径
	VirtualPath       string `json:"virtual_path"`
	VirtualTargetPath string `json:"virtual_target_path,omitempty"`
	FileSize          int64  `json:"file_size,omitempty"` //文件大小
	FsProvider        int    `json:"fs_provider"`         //用户文件系统类型
	Bucket            string `json:"bucket,omitempty"`    //
	Endpoint          string `json:"endpoint,omitempty"`
	IP                string `json:"ip"`         //用户IP
	SessionID         string `json:"session_id"` //用户会话ID
	Timestamp         int64  `json:"timestamp"`  //时间戳，用户操作完成时间戳
}

type OssConfig struct {
	Bucket       string `json:"bucket" mapstructure:"bucket"`
	Region       string `json:"region" mapstructure:"region"`
	AccessKey    string `json:"access_key" mapstructure:"access_key"`
	AccessSecret string `json:"access_secret" mapstructure:"access_secret"`
	Endpoint     string `json:"endpoint" mapstructure:"endpoint"`
}

type BackupPath struct {
	OSS string `json:"oss" mapstructure:"oss"`
	OS  string `json:"os" mapstructure:"os"`
}

type ProviderConfig struct {
	OSS OssConfig `json:"oss" mapstructure:"oss"`
}

//Config venus配置项
type Config struct {
	//Enable 是否启用流转功能
	Enable         bool           `json:"enable" mapstructure:"enable"`
	ProviderConfig ProviderConfig `json:"provider_config" mapstructure:"provider_config"`
	BackupPath     BackupPath     `json:"backup_path" mapstructure:"backup_path"`
	//Timeout 流转超时时间
	Timeout int `json:"timeout" mapstructure:"timeout"`
	//机器IP
	Addr string `json:"addr" mapstructure:"addr"`
}

type fss struct {
	OssFs vfs.Fs
	OsFs  vfs.Fs
}

var (
	msgChan       chan *Msg
	serviceStatus bool
)
var venusConfig Config

func (c *Config) Initialize(configDir string) error {
	if !c.Enable {
		return fmt.Errorf("%v", "流转程序功能未开启")
	}
	msgChan = make(chan *Msg, 100)
	serviceStatus = true
	//设置配置文件
	venusConfig = *c
	//定时器，用于维护fs连接是否可用等
	startVFSKeepaliveTimer()
	startTaskListener()
	return nil
}

func startTaskListener() {
	go func() {
		for {
			select {
			case msg := <-msgChan:
				task := NewDistribution(msg)
				task.run(GetVenusTimeout())
			}
		}
	}()
}

func getOssFsByS3Config(config vfs.S3FsConfig) (vfs.Fs, error) {
	if err := config.AccessSecret.TryDecrypt(); err != nil {
		return nil, fmt.Errorf("公共OSS密码解密失败，无法初始化OSS")
	}
	aliConfig := &vfs.AliOSSFsConfig{
		BaseS3FsConfig:        config.BaseS3FsConfig,
		AccessSecretPlaintext: config.AccessSecret.GetPayload(),
	}
	ossFs, err := vfs.NewAliOSSFs("", "", "", *aliConfig, false)
	if err != nil {
		return nil, fmt.Errorf("OSS 初始化失败，请检查配置")
	}
	return ossFs, nil
}

//远程连接保活
func startVFSKeepaliveTimer() {

}

//流转日志
func printLog(connectionID string, log interface{}) {
	logger.Info(logSender, connectionID, "%v", log)
}

func AcceptTask(event *notifier.FsEvent) error {
	//如果没有开启流转功能的话，返回
	logger.ErrorToConsole("开始接受消息: %#v", *event)
	msg := &Msg{
		Action:            event.Action,
		Username:          event.Username,
		Path:              event.Path,
		TargetPath:        event.TargetPath,
		VirtualPath:       event.VirtualPath,
		VirtualTargetPath: event.VirtualTargetPath,
		FileSize:          event.FileSize,
		FsProvider:        event.FsProvider,
		Bucket:            event.Bucket,
		Endpoint:          event.Endpoint,
		IP:                event.IP,
		SessionID:         event.SessionID,
		Timestamp:         event.Timestamp,
	}
	msgChan <- msg
	return nil
}

// GetStatus returns the server status
func isStarting() bool {
	return serviceStatus
}

func IsEnable() bool {
	return venusConfig.Enable
}

func GetVenusConfig() Config {
	return venusConfig
}

func GetVenusTimeout() int {
	return venusConfig.Timeout
}
