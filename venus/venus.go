package venus

import (
	"fmt"
)

const (
	logSender = "venus"
)

type modeType int

const (
	ModeLocal2Local modeType = iota
	ModeLocal2Remote
	ModeRemote2Local
)

var (
	distributionChan chan distribution
)

type VService interface {
	DistributionFile() (bool, error)
}

type Config struct {
	// Enable 是否启用流转功能
	Enable bool `json:"enable" mapstructure:"enable"`
}

type move interface {
	local2local()
}

var venusConfig Config

func (c *Config) Initialize(configDir string) error {
	if !c.Enable {
		return fmt.Errorf("%v", "流转程序功能未开启")
	}
	distributionChan = make(chan distribution, 10)
	// TODO: 定时任务线程启动
	return nil
}

func GetDistributionTask(fileName string) *VService {
	return nil
}

//todo 这里写vfs支持的功能，流转文件的开始
