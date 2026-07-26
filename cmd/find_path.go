// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// It defines all commands (root, list, get, tree) and their flags, delegating
// business logic to the services package.
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
)

var (
	findPathLnPattern    string
	findPathPath         string
	findPathIncludeDas   bool
	findPathDetailedFlag bool
)

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Find paths matching criteria",
}

var findPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Find paths matching logical node and data object criteria",
	Long: `Searches across all logical devices for logical nodes matching a pattern
and data objects matching exactly. Outputs full paths in the format: <ld>/<ln>.<do>

The --ln flag accepts a regex pattern (e.g., "MMXU" will match "VAMMXU1").
The --path flag specifies the data object path (DO or DO.DA format, exact match).

Examples:
  iec61850ctl find path --ln MMXU --path Hz
  iec61850ctl find path --ln MMXU --path A
  iec61850ctl find path --ln MMXU --path A --include-das
  iec61850ctl find path --ln MMXU --path A.phsA --include-das
  iec61850ctl find path --ln MMXU --path A.phsA --include-das --detailed`,
	RunE: runFindPath,
}

func init() {
	findPathCmd.Flags().StringVar(&findPathLnPattern, "ln", "", "logical node pattern (regex, required)")
	findPathCmd.Flags().StringVar(&findPathPath, "path", "", "data object path (DO or DO.DA format, exact match, required)")
	findPathCmd.Flags().BoolVar(&findPathIncludeDas, "include-das", false, "include all leaf data attributes")
	findPathCmd.Flags().BoolVar(&findPathDetailedFlag, "detailed", false, "show detailed information including DA details (FC, Type, Value)")
	_ = findPathCmd.MarkFlagRequired("ln")
	_ = findPathCmd.MarkFlagRequired("path")

	findCmd.AddCommand(findPathCmd)
	rootCmd.AddCommand(findCmd)
}

// runFindPath executes the 'find path' command to search for matching paths.
// It delegates all business logic to the explorer service and formats the output.
func runFindPath(cmd *cobra.Command, args []string) error {
	// Parse path to extract DO and DA parts
	// Format: "DO" or "DO.DA" or "DO.DA.nested"
	pathParts := strings.SplitN(findPathPath, ".", 2)
	doName := pathParts[0]
	daName := ""
	if len(pathParts) > 1 {
		daName = pathParts[1]
	}

	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	a := app.New(conn)
	result, err := a.FindPath(app.FindPathInput{
		LNPattern:  findPathLnPattern,
		DOName:     doName,
		DAName:     daName,
		IncludeDAs: findPathIncludeDas,
		Detailed:   findPathDetailedFlag,
	})
	if err != nil {
		return err
	}

	// Output results
	if len(result.Matches) == 0 {
		fmt.Printf("No matches found for LN pattern '%s' and path '%s'\n", findPathLnPattern, findPathPath)
		return nil
	}

	// Output each match
	for _, match := range result.Matches {
		basePath := fmt.Sprintf("%s/%s.%s", match.LD, match.LN, match.DO)

		// When a DA is specified and --include-das is not set, only print the DO.DA path.
		if daName != "" {
			fmt.Printf("%s.%s\n", basePath, daName)
		} else {
			fmt.Println(basePath)
		}

		// If including DAs, output them
		if findPathIncludeDas && match.DataAttributes != nil {
			for parentName, attributes := range match.DataAttributes {
				for _, attr := range attributes {
					var daPath string
					if attr.Name == "" {
						daPath = fmt.Sprintf("%s.%s", basePath, parentName)
					} else {
						daPath = fmt.Sprintf("%s.%s.%s", basePath, parentName, attr.Name)
					}

					if findPathDetailedFlag {
						// Detailed mode: show FC, Type, Value
						fcStr := "?"
						if attr.FC != "" {
							fcStr = attr.FC.String()
						}
						// Format attribute value inline (formatter cannot import domain)
						valueStr := "(nil)"
						if attr.ValueError != "" {
							valueStr = fmt.Sprintf("<error: %s>", attr.ValueError)
						} else if attr.Value != nil {
							if attr.Value.Display != "" {
								valueStr = attr.Value.Display
							} else {
								valueStr = attr.Value.String()
							}
						}
						fmt.Printf("  %s [FC=%s] Type: %s Value: %s\n",
							daPath, fcStr, attr.Type.String(), valueStr)
					} else {
						// Simple mode: just show path
						fmt.Printf("  %s\n", daPath)
					}
				}
			}
		}
	}

	// Always show call count
	fmt.Printf("\nTotal IEC61850 calls made: %d\n", result.CallCount)

	return nil
}
