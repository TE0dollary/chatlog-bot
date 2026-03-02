package glance

import (
	"fmt"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/rs/zerolog/log"
)

type Glance struct {
	PID        uint32
	MemRegions []MemRegion
	data       []byte
}

func NewGlance(pid uint32) *Glance {
	return &Glance{
		PID: pid,
	}
}

func (g *Glance) Read() ([]byte, error) {
	if g.data != nil {
		return g.data, nil
	}

	regions, err := GetVmmap(g.PID)
	if err != nil {
		return nil, err
	}
	g.MemRegions = MemRegionsFilter(regions)

	if len(g.MemRegions) == 0 {
		return nil, errors.ErrNoMemoryRegionsFound
	}

	// 遍历所有内存区域，尝试读取
	for i, region := range g.MemRegions {
		size := region.End - region.Start
		log.Info().
			Int("region_index", i).
			Int("total_regions", len(g.MemRegions)).
			Str("region_type", region.RegionType).
			Str("start", fmt.Sprintf("0x%x", region.Start)).
			Str("end", fmt.Sprintf("0x%x", region.End)).
			Uint64("size", size).
			Msg("尝试读取内存区域")

		data, err := g.readRegion(region)
		if err != nil {
			log.Warn().
				Int("region_index", i).
				Err(err).
				Msg("读取内存区域失败，尝试下一个区域")
			continue
		}

		g.data = data
		log.Info().
			Int("region_index", i).
			Int("data_size", len(data)).
			Msg("成功读取内存区域")
		return g.data, nil
	}

	return nil, errors.ErrNoMemoryRegionsFound
}

// readRegion 使用 Mach VM API 直接读取单个内存区域（替代原有的 lldb 方案）
func (g *Glance) readRegion(region MemRegion) ([]byte, error) {
	size := region.End - region.Start
	return MachReadMemory(g.PID, region.Start, size)
}
