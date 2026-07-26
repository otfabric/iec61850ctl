// SPDX-License-Identifier: MIT

package domain

import (
	"fmt"
	"strings"
)

// ObjectReference is a parsed IEC 61850 MMS object reference.
// Format: LD/LN.DO.DA.subDA (e.g., "ZX2REX640A1LD0/FMMXU1.Hz.mag.f").
type ObjectReference struct {
	LD   string
	LN   string
	Path []string
	FC   FunctionalConstraint
}

// ParseObjectReference parses a string like "LD/LN.DO.DA.subDA" into an ObjectReference.
func ParseObjectReference(ref string) (ObjectReference, error) {
	r := ObjectReference{}

	slashIdx := strings.Index(ref, "/")
	if slashIdx < 0 {
		return r, fmt.Errorf("invalid reference %q: missing '/' separator", ref)
	}

	r.LD = ref[:slashIdx]
	rest := ref[slashIdx+1:]

	if rest == "" {
		return r, fmt.Errorf("invalid reference %q: missing logical node after '/'", ref)
	}

	dotIdx := strings.Index(rest, ".")
	if dotIdx < 0 {
		r.LN = rest
		return r, nil
	}

	r.LN = rest[:dotIdx]
	pathStr := rest[dotIdx+1:]
	if pathStr != "" {
		r.Path = strings.Split(pathStr, ".")
	}
	return r, nil
}

// Build constructs an MMS reference string from components.
func Build(ld, ln string, path ...string) string {
	ref := ld + "/" + ln
	if len(path) > 0 {
		ref += "." + strings.Join(path, ".")
	}
	return ref
}

// String returns the canonical string form of the reference.
func (r ObjectReference) String() string {
	return Build(r.LD, r.LN, r.Path...)
}

// DOName returns the data object name (first path segment), or "" if no path.
func (r ObjectReference) DOName() string {
	if len(r.Path) > 0 {
		return r.Path[0]
	}
	return ""
}

// DAPath returns the data attribute path segments (everything after the DO name).
func (r ObjectReference) DAPath() []string {
	if len(r.Path) > 1 {
		return r.Path[1:]
	}
	return nil
}

// WithFC returns a copy of the reference with the given FC set.
func (r ObjectReference) WithFC(fc FunctionalConstraint) ObjectReference {
	r.FC = fc
	return r
}

// DataSetRef constructs a domain-style data set reference: "LD/LN$dsName".
func DataSetRef(ld, ln, dsName string) string {
	return ld + "/" + ln + "$" + dsName
}

// ReportRef constructs an RCB reference: "LD/LN.FC.rcbName".
func ReportRef(ld, ln string, fc FunctionalConstraint, rcbName string) string {
	return ld + "/" + ln + "." + string(fc) + "." + rcbName
}

// JournalRef constructs a journal/log reference: "LD/LN$logName".
func JournalRef(ld, ln, logName string) string {
	return ld + "/" + ln + "$" + logName
}

// ParseDataSetRef parses an RCB DatSet attribute value into LD, LN, and data set name.
// Accepts domain format (LD/LN$Name) or directory format (LD/LN.Name).
// Returns ok false if the string cannot be parsed.
func ParseDataSetRef(datSet string) (ld, ln, dsName string, ok bool) {
	if datSet == "" {
		return "", "", "", false
	}
	slash := strings.Index(datSet, "/")
	if slash <= 0 || slash == len(datSet)-1 {
		return "", "", "", false
	}
	ld = datSet[:slash]
	rest := datSet[slash+1:]
	sep := strings.IndexAny(rest, "$.")
	if sep <= 0 || sep == len(rest)-1 {
		return "", "", "", false
	}
	ln = rest[:sep]
	dsName = rest[sep+1:]
	return ld, ln, dsName, true
}

// ParseReportRef parses an RCB reference like "LD/LN.FC.rcbName" into its components.
// Returns ok false if the string cannot be parsed.
func ParseReportRef(reportRef string) (ld, ln string, fc FunctionalConstraint, rcbName string, ok bool) {
	if reportRef == "" {
		return "", "", "", "", false
	}
	slash := strings.Index(reportRef, "/")
	if slash <= 0 || slash == len(reportRef)-1 {
		return "", "", "", "", false
	}
	ld = reportRef[:slash]
	rest := reportRef[slash+1:]

	// LN.FC.rcbName
	parts := strings.SplitN(rest, ".", 3)
	if len(parts) != 3 {
		return "", "", "", "", false
	}
	ln = parts[0]
	fc = FunctionalConstraint(parts[1])
	rcbName = parts[2]

	if ln == "" || rcbName == "" {
		return "", "", "", "", false
	}
	return ld, ln, fc, rcbName, true
}

// ParseJournalRef parses a journal reference like "LD/LN$logName" into its components.
// Returns ok false if the string cannot be parsed.
func ParseJournalRef(journalRef string) (ld, ln, logName string, ok bool) {
	if journalRef == "" {
		return "", "", "", false
	}
	slash := strings.Index(journalRef, "/")
	if slash <= 0 || slash == len(journalRef)-1 {
		return "", "", "", false
	}
	ld = journalRef[:slash]
	rest := journalRef[slash+1:]

	ln, logName, okCut := strings.Cut(rest, "$")
	if !okCut || ln == "" || logName == "" {
		return "", "", "", false
	}
	return ld, ln, logName, true
}

// Parent returns a new ObjectReference with the last path segment removed.
func (r ObjectReference) Parent() ObjectReference {
	p := ObjectReference{LD: r.LD, LN: r.LN, FC: r.FC}
	if len(r.Path) > 1 {
		p.Path = make([]string, len(r.Path)-1)
		copy(p.Path, r.Path[:len(r.Path)-1])
	}
	return p
}

// Child returns a new ObjectReference with an additional path segment appended.
func (r ObjectReference) Child(segment string) ObjectReference {
	c := ObjectReference{LD: r.LD, LN: r.LN, FC: r.FC}
	c.Path = make([]string, len(r.Path)+1)
	copy(c.Path, r.Path)
	c.Path[len(r.Path)] = segment
	return c
}
