package venus

import (
	"context"
	"fmt"
	"github.com/drakkan/sftpgo/v2/dataprovider"
	"path"
	"strings"
	"time"
)

type VService interface {
	// 检查文件
	checkFile(ctx context.Context) error
	//备份文件
	backupFile(ctx context.Context) error
	// 分发文件
	distributionFile(ctx context.Context) error
	//删除文件
	deleteFile(ctx context.Context) error
	//启动
	run(ctx context.Context)
}

type distribution struct {
	msg  *Msg
	rule *dataprovider.VenusRule
	log  distributionLog
}

type distributionLog struct {
	//流水号，一对一和一对多,雪花id
	transID string
	//文件名
	fileName string
	fileSize int64
	//工作机ip端口
	workerAddr string
	//开始时间，精确到毫秒
	startTime int64
	//结束时间，精确到毫秒
	endTime int64
	//规则模式
	ruleMode      int
	ruleCode      string
	ruleErr       string
	ruleMatchTime int64
	backupEnable  bool
	backupSource  string
	backupTarget  string
	backupErr     string
	backupTime    int64
	backupSize    int64
	osn           sn
	hsn           []sn
	deleteEnable  bool
	deleteSource  string
	deleteErr     string
	deleteTime    int64
	err           string
}
type sn struct {
	username string
	filepath string
	provider int
	addr     string
	datetime int64
	err      string
}

func NewDistribution(msg *Msg) *distribution {
	return &distribution{
		msg: msg,
		log: distributionLog{transID: ""},
	}
}

func (d *distribution) run(timeout int) {
	ctx, canalFunc := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer canalFunc()
	d.log.startTime = time.Now().UnixMilli()
	go func() {
		defer canalFunc()
		//匹配规则
		err := d.setVenusRule()
		d.setVenusRuleLog(err)
		if err != nil {
			d.log.err = err.Error()
			return
		}
		vs, err := d.getVService()
		if err != nil {
			d.log.err = err.Error()
			return
		}
		vs.run(ctx)
	}()
	//超时等待上下文结束
	select {
	case <-ctx.Done():
		d.log.endTime = time.Now().UnixMilli()
		printLog(d.msg.SessionID, d.log)
	}
}

func (d *distribution) setVenusRule() error {
	if d.msg.Username == "" {
		return fmt.Errorf("用户为空，无法查询规则")
	}
	rules, err := dataprovider.GetRulesByOsn(d.msg.Username)
	if err != nil {
		return err
	}
	//match rule
	rule, err := d.matchPattern(rules)
	if err != nil {
		d.log.ruleErr = err.Error()
		return err
	}
	d.rule = rule
	return nil
}

func (d *distribution) getVService() (VService, error) {
	if d.rule == nil {
		return nil, fmt.Errorf("流转方式Mode: %v 未知", d.rule.Mode)
	}
	switch d.rule.Mode {
	case dataprovider.VenusRuleModeLocal2Mix:
		return newLocal2Mix(d), nil
	case dataprovider.VenusRuleModeRemote2Local:
		//TODO 待添加远端到本地流转
	}
	return nil, fmt.Errorf("流转方式Mode: %v 未知", d.rule.Mode)
}

func (d *distribution) matchPattern(rules []dataprovider.VenusRule) (*dataprovider.VenusRule, error) {
	if d.msg.VirtualPath != "" {
		toMatch := strings.ToLower(path.Base(d.msg.VirtualPath))
		for _, rule := range rules {
			if rule.Pattern != "" {
				matched, err := path.Match(rule.Pattern, toMatch)
				if err == nil && matched {
					return &rule, nil
				}
			}
		}

	}
	return nil, fmt.Errorf("未找到对应规则！")
}

func (d *distribution) setVenusRuleLog(err error) {
	if err != nil {
		d.log.ruleErr = err.Error()
	} else if d.rule != nil {
		d.log.ruleMatchTime = time.Now().UnixMilli()
		d.log.ruleCode = d.rule.Code
		d.log.ruleMode = d.rule.Mode
	}
	return
}
