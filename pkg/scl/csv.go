// SPDX-License-Identifier: MIT

package scl

import (
	"io"
	"strings"
)

// EscapeCSVField quotes the field if it contains the separator, newline, or double quote.
func EscapeCSVField(s, sep string) string {
	if s == "" {
		return s
	}
	if strings.Contains(s, sep) || strings.ContainsAny(s, "\"\r\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// CSVHeaders are the column headers for SCL flatten CSV output.
const CSVHeaders = "Logical Device,Logical Node,Data Object,Data Attribute,Function Code,Value,Type,Enum"

// WriteCSV writes flattened SCL entries as CSV to w. Separator is used between fields (e.g. "," or "|").
func WriteCSV(w io.Writer, sep string, entries []FlattenEntry) error {
	if _, err := io.WriteString(w, strings.ReplaceAll(CSVHeaders, ",", sep)+"\n"); err != nil {
		return err
	}
	for _, e := range entries {
		ld, ln, do, da, fc, val, bType, enum := CSVRow(e)
		fields := []string{
			EscapeCSVField(ld, sep),
			EscapeCSVField(ln, sep),
			EscapeCSVField(do, sep),
			EscapeCSVField(da, sep),
			EscapeCSVField(fc, sep),
			EscapeCSVField(val, sep),
			EscapeCSVField(bType, sep),
			EscapeCSVField(enum, sep),
		}
		if _, err := io.WriteString(w, strings.Join(fields, sep)+"\n"); err != nil {
			return err
		}
	}
	return nil
}
