//go:build !darwin

package glance

import "fmt"

// MachReadMemory 在非 macOS 平台上返回不支持错误。
// Mach VM API 仅在 macOS (darwin) 上可用。
func MachReadMemory(pid uint32, address, size uint64) ([]byte, error) {
	return nil, fmt.Errorf("Mach VM 内存读取仅在 macOS 平台上支持")
}
