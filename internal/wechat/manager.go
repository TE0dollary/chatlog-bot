package wechat

import (
	"context"
	"runtime"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/decrypt"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/key"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/process"
)

// Manager 微信管理器
type Manager struct {
	detector  process.Detector
	processes []*model.Process
}

// NewManager 创建新的微信管理器
func NewManager() *Manager {
	m := &Manager{
		detector: process.NewDetector(runtime.GOOS),
	}
	_ = m.Load()
	return m
}

// Load 加载微信进程信息
func (m *Manager) Load() error {
	processes, err := m.detector.FindProcesses()
	if err != nil {
		return err
	}
	m.processes = processes
	return nil
}

// GetProcess 按名称查找进程（先刷新进程列表）
func (m *Manager) GetProcess(name string) (*model.Process, error) {
	if err := m.Load(); err != nil {
		return nil, err
	}
	for _, p := range m.processes {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, errors.WeChatAccountNotFound(name)
}

// GetProcesses 获取所有微信进程
func (m *Manager) GetProcesses() []*model.Process {
	return m.processes
}

// ExtractKey 从指定账号的进程内存中提取加密密钥
func (m *Manager) ExtractKey(ctx context.Context, name string) (string, map[string]string, error) {
	proc, err := m.GetProcess(name)
	if err != nil {
		return "", nil, err
	}

	if proc.Status != model.StatusOnline {
		return "", nil, errors.WeChatAccountNotOnline(name)
	}

	extractor, err := key.NewExtractor(proc.Platform, proc.Version)
	if err != nil {
		return "", nil, err
	}

	validator, err := decrypt.NewValidator(proc.Platform, proc.Version, proc.DataDir)
	if err != nil {
		return "", nil, err
	}

	extractor.SetValidate(validator)

	imgKey, derivedKeyMap, err := extractor.Extract(ctx, proc)
	if err != nil && len(derivedKeyMap) == 0 {
		return "", nil, err
	}

	return imgKey, derivedKeyMap, nil
}
