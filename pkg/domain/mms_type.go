// SPDX-License-Identifier: MIT

package domain

import "github.com/otfabric/go-mms"

// MmsDataType is a string-typed enum for MMS value types.
type MmsDataType string

const (
	TypeArray           MmsDataType = "ARRAY"
	TypeStructure       MmsDataType = "STRUCT"
	TypeBoolean         MmsDataType = "BOOL"
	TypeBitString       MmsDataType = "BIT_STRING"
	TypeInteger         MmsDataType = "INT"
	TypeUnsigned        MmsDataType = "UINT"
	TypeFloat           MmsDataType = "FLOAT"
	TypeOctetString     MmsDataType = "OCTET_STRING"
	TypeVisibleString   MmsDataType = "STRING"
	TypeGeneralizedTime MmsDataType = "GENERALIZED_TIME"
	TypeBinaryTime      MmsDataType = "BINARY_TIME"
	TypeBcd             MmsDataType = "BCD"
	TypeObjId           MmsDataType = "OBJ_ID"
	TypeMmsString       MmsDataType = "MMS_STRING"
	TypeUtcTime         MmsDataType = "UTC_TIME"
	TypeDataAccessError MmsDataType = "DATA_ACCESS_ERROR"
	TypeInt8            MmsDataType = "INT8"
	TypeInt16           MmsDataType = "INT16"
	TypeInt32           MmsDataType = "INT32"
	TypeInt64           MmsDataType = "INT64"
	TypeUint8           MmsDataType = "UINT8"
	TypeUint16          MmsDataType = "UINT16"
	TypeUint32          MmsDataType = "UINT32"
	TypeUnknown         MmsDataType = "UNKNOWN"
)

func (t MmsDataType) String() string {
	if t == "" {
		return "UNKNOWN"
	}
	return string(t)
}

func (t MmsDataType) IsLeaf() bool {
	return t != TypeStructure && t != TypeArray && t != TypeUnknown && t != ""
}

func (t MmsDataType) IsNumeric() bool {
	switch t {
	case TypeInteger, TypeUnsigned, TypeFloat, TypeBcd,
		TypeInt8, TypeInt16, TypeInt32, TypeInt64,
		TypeUint8, TypeUint16, TypeUint32:
		return true
	default:
		return false
	}
}

func (t MmsDataType) IsString() bool {
	switch t {
	case TypeVisibleString, TypeMmsString, TypeObjId:
		return true
	default:
		return false
	}
}

func (t MmsDataType) IsTime() bool {
	switch t {
	case TypeUtcTime, TypeBinaryTime, TypeGeneralizedTime:
		return true
	default:
		return false
	}
}

// FromMMSValueType converts a go-mms ValueType to MmsDataType.
func FromMMSValueType(t mms.ValueType) MmsDataType {
	switch t {
	case mms.ValueTypeArray:
		return TypeArray
	case mms.ValueTypeStructure:
		return TypeStructure
	case mms.ValueTypeBoolean:
		return TypeBoolean
	case mms.ValueTypeBitString:
		return TypeBitString
	case mms.ValueTypeInteger:
		return TypeInteger
	case mms.ValueTypeUnsigned:
		return TypeUnsigned
	case mms.ValueTypeFloat, mms.ValueTypeReal:
		return TypeFloat
	case mms.ValueTypeOctetString:
		return TypeOctetString
	case mms.ValueTypeVisibleString:
		return TypeVisibleString
	case mms.ValueTypeGeneralizedTime:
		return TypeGeneralizedTime
	case mms.ValueTypeBinaryTime:
		return TypeBinaryTime
	case mms.ValueTypeBCD:
		return TypeBcd
	case mms.ValueTypeObjectIdentifier:
		return TypeObjId
	case mms.ValueTypeMmsString:
		return TypeMmsString
	case mms.ValueTypeUTCTime:
		return TypeUtcTime
	case mms.ValueTypeDataAccessError:
		return TypeDataAccessError
	default:
		return TypeUnknown
	}
}

// FromLibMmsType is retained as an alias for FromMMSValueType.
func FromLibMmsType(t mms.ValueType) MmsDataType {
	return FromMMSValueType(t)
}
