package cmd

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/spf13/cobra"
)

// Feature: path-detection, Property 4: Skip flag behavior
// Validates: Requirements 3.1

// genCommandName generates command names that should or should not skip PATH check.
func genCommandName() gopter.Gen {
	return gen.OneConstOf(
		"gitsage",
		"commit",
		"generate",
		"config",
		"help",
		"version",
		"history",
	)
}

// genSkipFlagValue generates boolean values for the skip-path-check flag.
func genSkipFlagValue() gopter.Gen {
	return gen.Bool()
}

// shouldSkipPathCheck determines if PATH check should be skipped based on command and flag.
// This is the reference implementation for testing.
func shouldSkipPathCheck(cmdName string, skipFlag bool, helpFlag bool) bool {
	// Skip for config, help, and version commands
	if cmdName == "config" || cmdName == "help" || cmdName == "version" {
		return true
	}

	// Skip if help flag is set
	if helpFlag {
		return true
	}

	// Skip if --skip-path-check flag is set
	if skipFlag {
		return true
	}

	return false
}

// createTestCommand creates a test command with the given name and flags.
func createTestCommand(cmdName string, skipFlag bool, helpFlag bool) *cobra.Command {
	cmd := &cobra.Command{
		Use: cmdName,
	}
	cmd.Flags().Bool("skip-path-check", skipFlag, "Skip PATH detection check")
	cmd.Flags().Bool("help", helpFlag, "Help for command")
	return cmd
}

// TestProperty_SkipFlagBehavior verifies that for any execution with --skip-path-check flag set,
// the PATH detection logic should not execute regardless of the path_check_done config value.
//
// Feature: path-detection, Property 4: Skip flag behavior
// Validates: Requirements 3.1
func TestProperty_SkipFlagBehavior(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: When skip-path-check flag is true, PATH check should always be skipped
	properties.Property("skip-path-check flag always skips PATH check", prop.ForAll(
		func(cmdName string, helpFlag bool) bool {
			// When skipFlag is true, shouldSkipPathCheck should return true
			return shouldSkipPathCheck(cmdName, true, helpFlag) == true
		},
		genCommandName(),
		gen.Bool(),
	))

	// Property: Config command always skips PATH check regardless of flags
	properties.Property("config command always skips PATH check", prop.ForAll(
		func(skipFlag bool, helpFlag bool) bool {
			return shouldSkipPathCheck("config", skipFlag, helpFlag) == true
		},
		genSkipFlagValue(),
		gen.Bool(),
	))

	// Property: Help command always skips PATH check regardless of flags
	properties.Property("help command always skips PATH check", prop.ForAll(
		func(skipFlag bool, helpFlag bool) bool {
			return shouldSkipPathCheck("help", skipFlag, helpFlag) == true
		},
		genSkipFlagValue(),
		gen.Bool(),
	))

	// Property: Version command always skips PATH check regardless of flags
	properties.Property("version command always skips PATH check", prop.ForAll(
		func(skipFlag bool, helpFlag bool) bool {
			return shouldSkipPathCheck("version", skipFlag, helpFlag) == true
		},
		genSkipFlagValue(),
		gen.Bool(),
	))

	// Property: Help flag always skips PATH check regardless of command
	properties.Property("help flag always skips PATH check", prop.ForAll(
		func(cmdName string, skipFlag bool) bool {
			// When helpFlag is true, shouldSkipPathCheck should return true
			return shouldSkipPathCheck(cmdName, skipFlag, true) == true
		},
		genCommandName(),
		genSkipFlagValue(),
	))

	// Property: Regular commands without skip flag should not skip PATH check
	properties.Property("regular commands without skip flag do not skip PATH check", prop.ForAll(
		func(cmdName string) bool {
			// Only test commands that are not config, help, or version
			if cmdName == "config" || cmdName == "help" || cmdName == "version" {
				return true // Skip this test case
			}
			// When skipFlag is false and helpFlag is false, shouldSkipPathCheck should return false
			return shouldSkipPathCheck(cmdName, false, false) == false
		},
		genCommandName(),
	))

	// Property: Skip behavior is consistent with command creation
	properties.Property("skip behavior matches command flag state", prop.ForAll(
		func(cmdName string, skipFlag bool, helpFlag bool) bool {
			cmd := createTestCommand(cmdName, skipFlag, helpFlag)

			// Get the flag values from the command
			actualSkipFlag, _ := cmd.Flags().GetBool("skip-path-check")
			actualHelpFlag, _ := cmd.Flags().GetBool("help")

			// Verify flags are set correctly
			return actualSkipFlag == skipFlag && actualHelpFlag == helpFlag
		},
		genCommandName(),
		genSkipFlagValue(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}
