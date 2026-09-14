/*
Copyright © 2026 baosloan baosloan@gmail.com

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// 构建信息用包级 var，留给 -ldflags 在链接期注入
var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

// configPath 为 --config 参数，所有命令共用。
var configPath string

// rootCmd 表示不带任何子命令时调用的根命令。
var rootCmd = &cobra.Command{
	Use:   "composer",
	Short: "A Gin web service scaffold with single-node and cluster support",
	Long: `composer is a layered Gin web service.

Database, Redis and Kafka each support a single-node and a cluster topology,
selected by the ` + "`mode`" + ` field in their config block. Configuration is
validated at startup, so an incoherent topology fails immediately with a
precise message rather than as a connection timeout later.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute 将所有子命令添加到根命令中，并适当地设置标志。
// 该函数由 main.main() 调用，只需对 rootCmd 执行一次。
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to the config file (default: ./configs/config.yaml)")
}
