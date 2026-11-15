// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

var (
	fileName         string
	outputPath       string
	fileDetailedFlag bool
)

// getFileMaxBytes bounds the file get file holds in memory before it writes
// it to stdout or --output (256 MiB, the limit of go-iec61850's ReadFile).
// A larger file is stopped and closed on the server, nothing is written, and
// the command exits 10. Tests lower it.
var getFileMaxBytes = 256 << 20

// boundedBuffer is a bytes.Buffer that holds at most max bytes: a Write
// that would go beyond writes nothing and fails with
// iec61850.ErrLimitExceeded.
type boundedBuffer struct {
	bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.max-b.Len() {
		return 0, fmt.Errorf("%w: the file is larger than %d bytes, the most get file holds in memory", iec61850.ErrLimitExceeded, b.max)
	}
	return b.Buffer.Write(p)
}

var getFileCmd = &cobra.Command{
	Use:   "file",
	Short: "Download a file from the IEC 61850 server",
	Long: `Download file contents from the server's file directory.
If --output is not specified, the file content is written to stdout.
Use --detailed to see download metadata including file size and checksum.
The file is held in memory until it is complete, up to 256 MiB; a larger
file is stopped and nothing is written (exit 10).`,
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

	buf := &boundedBuffer{max: getFileMaxBytes}
	entry, err := app.New(conn).DownloadFile(fileName, buf)
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
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "File: %s\n", fileName)
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Size: %s\n", formatter.FormatFileSize(size))
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Checksum (SHA256): %x\n\n", sha256.Sum256(data))
		}
		// The file's bytes are data: they bypass the escaping of stdout.
		if _, err = rawWriter(cmd.OutOrStdout()).Write(data); err != nil {
			return fmt.Errorf("failed to write to stdout: %w", err)
		}
		return nil
	}

	if err = os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	if fileDetailedFlag {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %s -> %s (%s)\n", fileName, outputPath, formatter.FormatFileSize(uint64(len(data))))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Checksum (SHA256): %x\n", sha256.Sum256(data))
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %s -> %s\n", fileName, outputPath)
	}
	return nil
}
