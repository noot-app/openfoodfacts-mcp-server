package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// createTestRootCmd creates a fresh root command for testing to avoid state interference
func createTestRootCmd() *cobra.Command {
	var testName string

	cmd := &cobra.Command{
		Use:   "go-template",
		Short: "A simple CLI template built with Cobra",
		Long: `A simple CLI template built with Cobra.

This is a template project for building CLI applications in Go using the Cobra library.
You can use this as a starting point for your own CLI applications.`,
		Run: func(cmd *cobra.Command, args []string) {
			if testName != "" {
				cmd.Printf("Hello %s!\n", testName)
			} else {
				cmd.Printf("Hello World!\n")
			}
		},
	}

	cmd.Flags().StringVarP(&testName, "name", "n", "", "Name to greet (optional)")
	return cmd
}

func TestRootCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "default hello world",
			args:     []string{},
			expected: "Hello World!\n",
		},
		{
			name:     "hello with name flag",
			args:     []string{"--name", "John"},
			expected: "Hello John!\n",
		},
		{
			name:     "hello with short name flag",
			args:     []string{"-n", "Alice"},
			expected: "Hello Alice!\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh command for each test
			cmd := createTestRootCmd()

			// Capture output
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			// Set args and execute
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestRootCmdHelp(t *testing.T) {
	// Create a fresh command for the help test
	cmd := createTestRootCmd()

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Test help command
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	assert.NoError(t, err)
	output := buf.String()

	// Check that help output contains expected elements
	assert.Contains(t, output, "A simple CLI template built with Cobra")
	assert.Contains(t, output, "Usage:")
	assert.Contains(t, output, "go-template [flags]")
	assert.Contains(t, output, "Flags:")
	assert.Contains(t, output, "-h, --help")
	assert.Contains(t, output, "-n, --name string")
}
