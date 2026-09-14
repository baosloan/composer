package cmd

import (
	"composer/internal/config"
	"fmt"

	"github.com/spf13/cobra"
)

var configCheckCmd = &cobra.Command{
	Use:   "config:check",
	Short: "验证配置并打印解析后的拓扑结构",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		fmt.Printf("配置文件正确\n\n")
		fmt.Printf("app:       %s (%s), server mode %s\n",
			cfg.App.Name, cfg.App.Env, cfg.Server.Mode)
		fmt.Printf("listen:    %s%s\n", cfg.Server.Addr(), cfg.Server.BasePath)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCheckCmd)
}
