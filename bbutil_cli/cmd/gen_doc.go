/*
Copyright © 2023 LiuHailong
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// genDocCmd represents the genDoc command
var genDocCmd = &cobra.Command{
	Use:   "gdoc",
	Short: "Generate document",
	Long: `
Automatically generate command documentation for all commands
	`,
	Run: func(cmd *cobra.Command, args []string) {
		GenDosc(types)
	},
}

var types string

func init() {
	rootCmd.AddCommand(genDocCmd)

	//生成markdown
	genDocCmd.Flags().StringVarP(&types, "type", "t", "markdown", "markdown,rest or yaml")
	genDocCmd.MarkFlagRequired("mode")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// genDocCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// genDocCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
