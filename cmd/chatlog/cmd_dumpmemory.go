package chatlog

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/TE0dollary/chatlog-bot/internal/wechat"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/key/darwin/glance"
)

func init() {
	rootCmd.AddCommand(dumpmemoryCmd)
}

var dumpmemoryCmd = &cobra.Command{
	Use:   "dumpmemory",
	Short: "dump memory",
	Run: func(cmd *cobra.Command, args []string) {
		if runtime.GOOS != "darwin" {
			log.Info().Msg("dump memory only support macOS")
		}

		session := time.Now().Format("20060102150405")

		dir, err := os.Getwd()
		if err != nil {
			log.Fatal().Err(err).Msg("get current directory failed")
			return
		}
		log.Info().Msgf("current directory: %s", dir)

		// step 1. check pid
		m := wechat.NewManager()
		if err = m.Load(); err != nil {
			log.Fatal().Err(err).Msg("load wechat failed")
			return
		}
		processes := m.GetProcesses()
		if len(processes) == 0 {
			log.Fatal().Msg("no wechat process found")
			return
		}

		log.Info().Msgf("found %d wechat process", len(processes))
		for i, proc := range processes {
			log.Info().Msgf("%d. %s %d %s", i, proc.FullVersion, proc.PID, proc.DataDir)
		}

		// step 2. dump memory
		process := processes[0]
		file := fmt.Sprintf("wechat_%s_%d_%s.bin", process.FullVersion, process.PID, session)
		path := filepath.Join(dir, file)
		log.Info().Msgf("dumping memory to %s", path)

		g := glance.NewGlance(process.PID)
		b, err := g.Read()
		if err != nil {
			log.Fatal().Err(err).Msg("read memory failed")
			return
		}

		if err = os.WriteFile(path, b, 0644); err != nil {
			log.Fatal().Err(err).Msg("write memory failed")
			return
		}

		log.Info().Msg("dump memory success")

		// step 3. copy encrypted database file
		dbFile := "db_storage/session/session.db"
		if process.Version == 3 {
			dbFile = "Session/session_new.db"
		}
		from := filepath.Join(process.DataDir, dbFile)
		to := filepath.Join(dir, fmt.Sprintf("wechat_%s_%d_session.db", process.FullVersion, process.PID))

		log.Info().Msgf("copying %s to %s", from, to)
		b, err = os.ReadFile(from)
		if err != nil {
			log.Fatal().Err(err).Msg("read session.db failed")
			return
		}
		if err = os.WriteFile(to, b, 0644); err != nil {
			log.Fatal().Err(err).Msg("write session.db failed")
			return
		}
		log.Info().Msg("copy session.db success")

		// step 4. package
		zipFile := fmt.Sprintf("wechat_%s_%d_%s.zip", process.FullVersion, process.PID, session)
		zipPath := filepath.Join(dir, zipFile)
		log.Info().Msgf("packaging to %s", zipPath)

		zf, err := os.Create(zipPath)
		if err != nil {
			log.Fatal().Err(err).Msg("create zip file failed")
			return
		}
		defer zf.Close()

		zw := zip.NewWriter(zf)

		for _, file := range []string{file, to} {
			f, err := os.Open(file)
			if err != nil {
				log.Fatal().Err(err).Msg("open file failed")
				return
			}
			defer f.Close()
			info, err := f.Stat()
			if err != nil {
				log.Fatal().Err(err).Msg("get file info failed")
				return
			}
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				log.Fatal().Err(err).Msg("create zip file info header failed")
				return
			}
			header.Name = filepath.Base(file)
			header.Method = zip.Deflate
			writer, err := zw.CreateHeader(header)
			if err != nil {
				log.Fatal().Err(err).Msg("create zip file header failed")
				return
			}
			if _, err = io.Copy(writer, f); err != nil {
				log.Fatal().Err(err).Msg("copy file to zip failed")
				return
			}
		}
		if err = zw.Close(); err != nil {
			log.Fatal().Err(err).Msg("close zip writer failed")
			return
		}

		log.Info().Msgf("package success, please send %s to developer", zipPath)
	},
}
