// SPDX-License-Identifier: MIT

package scl

import (
	"encoding/xml"
	"io"
)

// SCL is the root element of IEC 61850 System Configuration Language.
type SCL struct {
	XMLName           xml.Name          `xml:"http://www.iec.ch/61850/2003/SCL SCL"`
	IED               []IED             `xml:"IED"`
	DataTypeTemplates DataTypeTemplates `xml:"DataTypeTemplates"`
}

// IED is an Intelligent Electronic Device in the SCL.
type IED struct {
	Name        string        `xml:"name,attr"`
	AccessPoint []AccessPoint `xml:"AccessPoint"`
}

// AccessPoint is a communication access point of an IED.
type AccessPoint struct {
	Name   string  `xml:"name,attr"`
	Server *Server `xml:"Server"`
}

// Server holds the logical devices of an access point.
type Server struct {
	LDevice []LDevice `xml:"LDevice"`
}

// LDevice is a logical device (LD) instance.
type LDevice struct {
	Inst string `xml:"inst,attr"`
	LN0  *LN0   `xml:"LN0"`
	LN   []LN   `xml:"LN"`
}

// LN0 is Logical Node Zero (LLN0) of a logical device.
type LN0 struct {
	Prefix        string          `xml:"prefix,attr"`
	LnClass       string          `xml:"lnClass,attr"`
	Inst          string          `xml:"inst,attr"`
	LnType        string          `xml:"lnType,attr"`
	DOI           []DOI           `xml:"DOI"`
	DataSet       []DataSet       `xml:"DataSet"`
	ReportControl []ReportControl `xml:"ReportControl"`
}

// DataSet is a list of FCDA references (dataset definition).
type DataSet struct {
	Name string `xml:"name,attr"`
	FCDA []FCDA `xml:"FCDA"`
}

// FCDA is a reference to a data attribute in a dataset.
type FCDA struct {
	LdInst  string `xml:"ldInst,attr"`
	Prefix  string `xml:"prefix,attr"`
	LnClass string `xml:"lnClass,attr"`
	LnInst  string `xml:"lnInst,attr"`
	DoName  string `xml:"doName,attr"`
	Fc      string `xml:"fc,attr"`
}

// ReportControl defines a report control block.
type ReportControl struct {
	Name     string `xml:"name,attr"`
	DatSet   string `xml:"datSet,attr"`
	Buffered string `xml:"buffered,attr"`
}

// LN is a logical node instance (non-LLN0).
type LN struct {
	Prefix  string `xml:"prefix,attr"`
	LnClass string `xml:"lnClass,attr"`
	Inst    string `xml:"inst,attr"`
	LnType  string `xml:"lnType,attr"`
	Desc    string `xml:"desc,attr"`
	DOI     []DOI  `xml:"DOI"`
}

// DOI is a Data Object instance (with optional DAI/SDI values).
type DOI struct {
	Name string `xml:"name,attr"`
	Desc string `xml:"desc,attr"`
	DAI  []DAI  `xml:"DAI"`
	SDI  []SDI  `xml:"SDI"`
}

// SDI is a Structure Data instance (nested under DOI).
type SDI struct {
	Name string `xml:"name,attr"`
	DAI  []DAI  `xml:"DAI"`
	SDI  []SDI  `xml:"SDI"`
}

// DAI is a Data Attribute instance (with optional Val).
type DAI struct {
	Name string `xml:"name,attr"`
	Val  string `xml:"Val"`
	DAI  []DAI  `xml:"DAI"`
	SDI  []SDI  `xml:"SDI"`
}

// DataTypeTemplates contains type definitions.
type DataTypeTemplates struct {
	LNodeType []LNodeType `xml:"LNodeType"`
	DOType    []DOType    `xml:"DOType"`
	DAType    []DAType    `xml:"DAType"`
	EnumType  []EnumType  `xml:"EnumType"`
}

// EnumType defines allowed values for an enumerated attribute.
type EnumType struct {
	ID      string    `xml:"id,attr"`
	EnumVal []EnumVal `xml:"EnumVal"`
}

// EnumVal is one allowed value (ord + text).
type EnumVal struct {
	Ord  string `xml:"ord,attr"`
	Text string `xml:",chardata"`
}

// LNodeType is a template defining the data objects of a logical node class.
type LNodeType struct {
	ID      string `xml:"id,attr"`
	LnClass string `xml:"lnClass,attr"`
	DO      []DO   `xml:"DO"`
}

// DO is a Data Object reference in an LNodeType.
type DO struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

// DOType is a template defining the structure (DA/SDO) of a data object.
type DOType struct {
	ID  string `xml:"id,attr"`
	CDC string `xml:"cdc,attr"`
	DA  []DA   `xml:"DA"`
	SDO []SDO  `xml:"SDO"`
}

// SDO is a sub-data object reference in a DOType.
type SDO struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

// DA is a Data Attribute definition in a DOType (or DAType).
type DA struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	BType string `xml:"bType,attr"`
	Fc    string `xml:"fc,attr"`
}

// DAType is a template defining the structure (BDA/DA) of a structured data type.
type DAType struct {
	ID  string `xml:"id,attr"`
	BDA []BDA  `xml:"BDA"`
	DA  []DA   `xml:"DA"`
}

// BDA is a Basic Data Attribute definition in a DAType.
type BDA struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	BType string `xml:"bType,attr"`
	Fc    string `xml:"fc,attr"`
}

// Parse reads an SCL/CID document from r.
func Parse(r io.Reader) (*SCL, error) {
	dec := xml.NewDecoder(r)
	var scl SCL
	if err := dec.Decode(&scl); err != nil {
		return nil, err
	}
	return &scl, nil
}

// appendPath returns a new SDI path with name appended, without mutating base.
func appendPath(base []string, name string) []string {
	out := make([]string, len(base), len(base)+1)
	copy(out, base)
	return append(out, name)
}
