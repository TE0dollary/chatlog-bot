package darwin

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
	"github.com/TE0dollary/chatlog-bot/pkg/appver"
)

const (
	ProcessNameOfficial = "WeChat"
	ProcessNameBeta     = "Weixin"
	V4DBFile            = "db_storage/session/session.db"
)

// Detector implements the macOS process detector.
type Detector struct{}

// NewDetector creates a new macOS process detector.
func NewDetector() *Detector {
	return &Detector{}
}

// FindProcesses finds all WeChat processes and returns their info.
func (d *Detector) FindProcesses() ([]*model.Process, error) {
	processes, err := process.Processes()
	if err != nil {
		log.Err(err).Msg("获取进程列表失败")
		return nil, err
	}

	var result []*model.Process
	for _, p := range processes {
		name, err := p.Name()
		if err != nil || (name != ProcessNameOfficial && name != ProcessNameBeta) {
			continue
		}

		procInfo, err := d.getProcessInfo(p)
		if err != nil {
			log.Err(err).Msgf("获取进程 %d 的信息失败", p.Pid)
			continue
		}

		result = append(result, procInfo)
	}

	if len(result) == 0 {
		log.Debug().Msg("未发现运行中的微信进程")
	} else {
		for _, proc := range result {
			log.Info().
				Uint32("pid", proc.PID).
				Str("account", proc.Name).
				Str("version", proc.FullVersion).
				Str("status", string(proc.Status)).
				Msg("发现微信进程")
		}
	}

	return result, nil
}

// getProcessInfo retrieves detailed info for a WeChat process.
func (d *Detector) getProcessInfo(p *process.Process) (*model.Process, error) {
	procInfo := &model.Process{
		PID:      uint32(p.Pid),
		Status:   model.StatusOffline,
		Platform: model.PlatformMacOS,
	}

	exePath, err := p.Exe()
	if err != nil {
		log.Err(err).Msg("获取可执行文件路径失败")
		return nil, err
	}
	procInfo.ExePath = exePath

	versionInfo, err := appver.New(exePath)
	if err != nil {
		log.Err(err).Msg("获取版本信息失败")
		return nil, err
	}
	procInfo.Version = versionInfo.Version
	procInfo.FullVersion = versionInfo.FullVersion

	// init data directory and account name; return partial info on failure
	if err := d.initializeProcessInfo(p, procInfo); err != nil {
		log.Err(err).Msg("初始化进程信息失败")
	}

	return procInfo, nil
}

// initializeProcessInfo fetches the process data directory and account name.
func (d *Detector) initializeProcessInfo(p *process.Process, info *model.Process) error {
	files, err := d.getOpenFiles(int(p.Pid))
	if err != nil {
		log.Err(err).Msg("获取打开的文件失败")
		return err
	}

	dbPath := V4DBFile

	for _, filePath := range files {
		if strings.Contains(filePath, dbPath) {
			parts := strings.Split(filePath, string(filepath.Separator))
			if len(parts) < 4 {
				log.Debug().Msg("无效的文件路径格式: " + filePath)
				continue
			}

			// v4:
			// /Users/<username>/Library/Containers/com.tencent.xinWeChat/Data/Documents/xwechat_files/<wxid>/db_storage/message/message_0.db

			info.Status = model.StatusOnline
			info.DataDir = strings.Join(parts[:len(parts)-3], string(filepath.Separator))
			info.Name = parts[len(parts)-4]
			log.Debug().Str("account", info.Name).Str("data_dir", info.DataDir).Msg("进程数据目录已定位")
			return nil
		}
	}

	return nil
}

// getOpenFiles returns the list of open files for a process using lsof.
func (d *Detector) getOpenFiles(pid int) ([]string, error) {
	// -F n: output filenames only
	cmd := exec.Command("lsof", "-p", strconv.Itoa(pid), "-F", "n")
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.RunCmdFailed(err)
	}

	// parse lsof -F n output: each path line starts with 'n'
	lines := strings.Split(string(output), "\n")
	var files []string

	for _, line := range lines {
		if strings.HasPrefix(line, "n") {
			filePath := line[1:]
			if filePath != "" {
				files = append(files, filePath)
			}
		}
	}

	return files, nil
}
