//go:build darwin

package glance

/*
#include <mach/mach.h>
#include <mach/mach_vm.h>

// 获取目标进程的 task 端口
static kern_return_t get_task_port(int pid, mach_port_t *task) {
    return task_for_pid(mach_task_self(), pid, task);
}

// 使用 mach_vm_read_overwrite 直接读取进程内存到预分配缓冲区
static kern_return_t read_process_mem(mach_port_t task, uint64_t address, uint64_t size, void *buffer, uint64_t *out_size) {
    mach_vm_size_t sz = (mach_vm_size_t)size;
    kern_return_t kr = mach_vm_read_overwrite(task, (mach_vm_address_t)address, sz, (mach_vm_address_t)buffer, &sz);
    *out_size = (uint64_t)sz;
    return kr;
}

// 释放 task 端口
static void release_task_port(mach_port_t task) {
    mach_port_deallocate(mach_task_self(), task);
}
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/rs/zerolog/log"
)

// machReadChunkSize 每次 Mach VM 读取的块大小（4MB）
const machReadChunkSize = 4 * 1024 * 1024

// MachReadMemory 使用 macOS Mach VM API 直接读取目标进程的内存区域。
// 比 lldb 方案快 10~100 倍，无需启动调试器、加载符号或通过命名管道传输。
func MachReadMemory(pid uint32, address, size uint64) ([]byte, error) {
	var task C.mach_port_t
	kr := C.get_task_port(C.int(pid), &task)
	if kr != C.KERN_SUCCESS {
		return nil, fmt.Errorf("task_for_pid 失败 (pid=%d): kern_return_t=%d，请确认 SIP 已关闭且具有调试权限", pid, int(kr))
	}
	defer C.release_task_port(task)

	log.Debug().
		Uint32("pid", pid).
		Uint64("address", address).
		Uint64("size", size).
		Msg("开始 Mach VM 内存读取")

	// 预分配结果缓冲区
	result := make([]byte, 0, size)
	successChunks := 0
	failedChunks := 0

	for offset := uint64(0); offset < size; offset += machReadChunkSize {
		chunkSize := uint64(machReadChunkSize)
		if offset+chunkSize > size {
			chunkSize = size - offset
		}

		buf := make([]byte, chunkSize)
		var bytesRead C.uint64_t

		kr = C.read_process_mem(task, C.uint64_t(address+offset), C.uint64_t(chunkSize), unsafe.Pointer(&buf[0]), &bytesRead)
		if kr != C.KERN_SUCCESS {
			// 跳过不可读的块，用零填充以保持偏移量一致（密钥匹配依赖位置对齐）
			failedChunks++
			result = append(result, make([]byte, chunkSize)...)
			continue
		}

		successChunks++
		result = append(result, buf[:uint64(bytesRead)]...)
	}

	log.Debug().
		Int("success_chunks", successChunks).
		Int("failed_chunks", failedChunks).
		Int("total_bytes", len(result)).
		Msg("Mach VM 内存读取完成")

	if successChunks == 0 {
		return nil, fmt.Errorf("所有内存块均不可读 (address=0x%x, size=%d)", address, size)
	}

	return result, nil
}
