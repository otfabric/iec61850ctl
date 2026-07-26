// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

var (
	fileName         string
	outputPath       string
	fileDetailedFlag bool
)

var getFileCmd = &cobra.Command{
	Use:   "file",
	Short: "Download a file from the IEC 61850 server",
	Long: `Download file contents from the server's file directory.
If --output is not specified, the file content is written to stdout.
Use --detailed to see download metadata including file size and checksum.`,
	RunE: runGetFile,
}

func init() {
	getFileCmd.Flags().StringVar(&fileName, "name", "", "filename to download (required)")
	getFileCmd.Flags().StringVar(&outputPath, "output", "", "local file path to save (optional, defaults to stdout)")
	getFileCmd.Flags().BoolVar(&fileDetailedFlag, "detailed", false, "show download metadata (size, checksum)")
	_ = getFileCmd.MarkFlagRequired("name")
	getCmd.AddCommand(getFileCmd)
}

func runGetFile(cmd *cobra.Command, args []string) error {
	session, err := openClientSession(cmd, clientSessionOptions{RequestTimeout: 60 * time.Second})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	var buf bytes.Buffer
	entry, err := app.New(conn).DownloadFile(fileName, &buf)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	data := buf.Bytes()

	if outputPath == "" {
		if fileDetailedFlag {
			size := uint64(len(data))
			if entry != nil {
				size = uint64(entry.Size)
			}
			_, _ = fmt.Fprintf(os.Stderr, "File: %s\n", fileName)
			_, _ = fmt.Fprintf(os.Stderr, "Size: %s\n", formatter.FormatFileSize(size))
			_, _ = fmt.Fprintf(os.Stderr, "Checksum (SHA256): %x\n\n", sha256.Sum256(data))
		}
		if _, err = os.Stdout.Write(data); err != nil {
			return fmt.Errorf("failed to write to stdout: %w", err)
		}
		return nil
	}

	if err = os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	if fileDetailedFlag {
		fmt.Printf("Downloaded %s -> %s (%s)\n", fileName, outputPath, formatter.FormatFileSize(uint64(len(data))))
		fmt.Printf("Checksum (SHA256): %x\n", sha256.Sum256(data))
	} else {
		fmt.Printf("Downloaded %s -> %s\n", fileName, outputPath)
	}
	return nil
}
