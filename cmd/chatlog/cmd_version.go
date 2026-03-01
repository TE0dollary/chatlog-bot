package chatlog

import (
	"fmt"

	"github.com/TE0dollary/chatlog-bot/pkg/version"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVarP(&versionM, "module", "m", false, "module version information")
}

var versionM bool

// versionCmd 显示版本信息。-m 参数可显示更详细的模块版本。
var versionCmd = &cobra.Command{
	Use:   "version [-m]",
	Short: "Show the version of chatlog",
	Run: func(cmd *cobra.Command, args []string) {
		if versionM {
			fmt.Println(version.GetMore(true))
		} else {
			fmt.Printf("chatlog %s\n", version.GetMore(false))
		}
	},
}
