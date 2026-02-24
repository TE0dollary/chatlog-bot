package model

type Process struct {
	PID         uint32
	ExePath     string
	Platform    string
	Version     int
	FullVersion string
	Status      string
	DataDir     string
	Name        string
}

// 平台常量定义
const (
	PlatformMacOS = "darwin"
)

const (
	StatusOffline = "offline"
	StatusOnline  = "online"
)
