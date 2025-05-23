/*
Package cmd
Copyright © 2023 LiuHailong <lhl_creeper@163.com>
*/
package cmd

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "bup",
	Short:   "bbutil plus",
	Version: "1.0.0",
	Long:    `this is bbutil plus, we will become better.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func GenDosc(types string) {
	var dir, _ = os.Executable()
	var absPath, _ = filepath.Abs(filepath.Dir(dir))
	var path = strings.ReplaceAll(absPath, `\`, `/`)
	_, err := os.Stat(path + "/docs")
	if err != nil {
		os.Mkdir(path+"/docs/", 0644)
	}
	rootCmd.DisableAutoGenTag = true
	if types == "markdown" {
		err = doc.GenMarkdownTree(rootCmd, path+"/docs/")
		if err != nil {
			log.Fatal(err)
		}
		log.Println("The markdown document was successfully generated")
	} else if types == "rest" {
		err = doc.GenReSTTree(rootCmd, path+"/docs/")
		if err != nil {
			log.Fatal(err)
		}
		log.Println("The rest document was successfully generated")
	} else if types == "yaml" {
		err = doc.GenYamlTree(rootCmd, path+"/docs/")
		if err != nil {
			log.Fatal(err)
		}
		log.Println("The yaml document was successfully generated")
	}
}

func initConfig() {
	// common.Logger.Info("")
}
