package cmd

import (
	"github.com/spf13/cobra"
)

var (
	name string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-template",
	Short: "A simple CLI template built with Cobra",
	Long: `A simple CLI template built with Cobra.

This is a template project for building CLI applications in Go using the Cobra library.
You can use this as a starting point for your own CLI applications.`,
	Run: func(cmd *cobra.Command, args []string) {
		if name != "" {
			cmd.Printf("Hello %s!\n", name)
		} else {
			cmd.Printf("Hello World!\n")
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Define flags
	rootCmd.Flags().StringVarP(&name, "name", "n", "", "Name to greet (optional)")
}

// Run is the main entry point for the CLI application
func Run() error {
	return Execute()
}
