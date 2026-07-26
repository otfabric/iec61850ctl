// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

var (
	filesDetailedFlag  bool
	filesRecursiveFlag bool
	filesPathFlag      string
)

var listFilesCmd = &cobra.Command{
	Use:   "files",
	Short: "List files available on the IEC 61850 server",
	Long:  `List all files in the server's file directory. Use --detailed to see file metadata including size and last modified time.`,
	RunE:  runListFiles,
}

func init() {
	listFilesCmd.Flags().BoolVar(&filesDetailedFlag, "detailed", false, "show file metadata (size, last modified)")
	listFilesCmd.Flags().BoolVar(&filesRecursiveFlag, "recursive", false, "list files with path-depth indentation")
	listFilesCmd.Flags().StringVar(&filesPathFlag, "path", "", "directory path / filename pattern (optional)")
	listCmd.AddCommand(listFilesCmd)
}

func runListFiles(cmd *cobra.Command, args []string) error {
	finalHost, finalPort, err := getHostPort()
	if err != nil {
		return err
	}
	printConnectionTarget(finalHost, finalPort)

	conn, err := client.NewConnection(client.ConnectionInput{
		Host:           finalHost,
		Port:           finalPort,
		ConnectTimeout: 10,
		RequestTimeout: 30,
	})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.Background()) }()

	a := app.New(conn)
	entries, err := a.ListFiles(filesPathFlag)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	if filesRecursiveFlag {
		fmt.Printf("Files matching %q:\n\n", filesPathFlag)
		for _, entry := range entries {
			depth := strings.Count(entry.Name, "/") + strings.Count(entry.Name, "\\")
			indent := strings.Repeat("  ", depth)
			if filesDetailedFlag {
				fmt.Printf("%s%s (%s)\n", indent, entry.Name, formatter.FormatFileSize(uint64(entry.Size)))
			} else {
				fmt.Printf("%s%s\n", indent, entry.Name)
			}
		}
		return nil
	}

	pathInfo := "root directory"
	if filesPathFlag != "" {
		pathInfo = fmt.Sprintf("%q", filesPathFlag)
	}
	fmt.Printf("Found %d file(s) in %s:\n", len(entries), pathInfo)
	for i, entry := range entries {
		if filesDetailedFlag {
			fmt.Printf("  %d. %s\n", i+1, entry.Name)
			fmt.Printf("     Size: %s\n", formatter.FormatFileSize(uint64(entry.Size)))
			if !entry.LastModified.IsZero() {
				fmt.Printf("     Last Modified: %s\n", entry.LastModified.UTC().Format("2006-01-02T15:04:05Z"))
			}
			if i < len(entries)-1 {
				fmt.Println()
			}
		} else {
			fmt.Printf("  %d. %s\n", i+1, entry.Name)
		}
	}
	if !filesDetailedFlag {
		fmt.Println("\nUse --detailed flag to see file metadata")
	}
	return nil
}
