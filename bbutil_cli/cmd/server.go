/*
Package cmd
Copyright © 2023 LiuHailong <lhl_creeper@163.com>
*/
package cmd

import (
	"bbutil_cli/common"
	"bbutil_cli/server/router"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var port int64

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "bbutil plus web server",
	Long: `
  You can start a web service by running this command.
such as: use 'bbutil server --port=8080' or 'bbutil server -p 8080', 
We support accepting parameters in a variety of standard formats.
You can also specify the port of the service by defining server.port in the configuration file.
Of course, commands take precedence over configuration files.
	`,
	Run: func(cmd *cobra.Command, args []string) {
		if port == -1 {
			port = viper.GetInt64("server.port")
		}
		if port <= 0 || port > 65535 {
			common.Logger.Fatalf("the port %d is wrong", port)
		}
		common.Logger.Infof("the web port is %d", port)

		router.InitRouter(port)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().Int64VarP(&port, "port", "p", -1, `the server port, However, default value '-1' is not canonical,
you need to specify it again through a command or configuration file`)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serverCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serverCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
