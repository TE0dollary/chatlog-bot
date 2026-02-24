package process

import (
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/process/darwin"
)

type Detector interface {
	FindProcesses() ([]*model.Process, error)
}

// NewDetector 创建适合当前平台的检测器
func NewDetector(platform string) Detector {
	if platform == "darwin" {
		return darwin.NewDetector()
	}
	return &nullDetector{}
}

// nullDetector 空实现
type nullDetector struct{}

func (d *nullDetector) FindProcesses() ([]*model.Process, error) {
	return nil, nil
}

