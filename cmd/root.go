package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "dyff",
	Short: "A helm diff tool",
	Long:  `A helm diff tool`,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}
