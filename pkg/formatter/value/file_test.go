// SPDX-License-Identifier: MIT

package value

import "testing"

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes uint64
		want  string
	}{
		{"zero", 0, "0 bytes"},
		{"bytes only", 512, "512 bytes"},
		{"just under KB", 1023, "1023 bytes"},
		{"exact KB", 1024, "1.00 KB (1024 bytes)"},
		{"KB range", 1536, "1.50 KB (1536 bytes)"},
		{"exact MB", 1024 * 1024, "1.00 MB (1048576 bytes)"},
		{"MB range", 3 * 1024 * 1024 / 2, "1.50 MB (1572864 bytes)"},
		{"exact GB", 1024 * 1024 * 1024, "1.00 GB (1073741824 bytes)"},
		{"GB range", 2 * 1024 * 1024 * 1024, "2.00 GB (2147483648 bytes)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFileSize(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
