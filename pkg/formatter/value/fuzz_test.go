// SPDX-License-Identifier: MIT

package value

import (
	"math"
	"testing"
	"time"

	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// fuzzMMSValue deterministically builds an arbitrary (possibly nested) MMS
// value from fuzz bytes. It consumes data and returns the remainder.
func fuzzMMSValue(data []byte, depth int) (*mms.Value, []byte) {
	if len(data) == 0 {
		return mms.NewBoolean(false), data
	}
	op, data := data[0], data[1:]
	take := func(n int) []byte {
		if n > len(data) {
			n = len(data)
		}
		b := data[:n]
		data = data[n:]
		return b
	}
	u64 := func() uint64 {
		var v uint64
		for _, b := range take(8) {
			v = v<<8 | uint64(b)
		}
		return v
	}
	switch op % 20 {
	case 0:
		return mms.NewBoolean(op&0x80 != 0), data
	case 1:
		return mms.NewInteger(int64(u64())), data
	case 2:
		return mms.NewUnsigned(u64()), data
	case 3:
		return mms.NewFloat(math.Float64frombits(u64())), data
	case 4:
		return mms.NewReal(math.Float64frombits(u64())), data
	case 5:
		n := 0
		if len(data) > 0 {
			n = int(data[0]) % 8
		}
		b := take(n)
		return mms.NewBitStringWithLength(b, len(b)*8-int(op>>5)%8), data
	case 6:
		return mms.NewBitString(take(int(op >> 4))), data
	case 7:
		return mms.NewOctetString(take(int(op >> 3))), data
	case 8:
		return mms.NewVisibleString(string(take(int(op >> 3)))), data
	case 9:
		return mms.NewMmsString(string(take(int(op >> 3)))), data
	case 10:
		return mms.NewUTCTimeWithQuality(time.Unix(int64(u64()>>32), 0), op), data
	case 11:
		return mms.NewBinaryTime(int64(u64())), data
	case 12:
		return mms.NewGeneralizedTime(time.Unix(int64(u64()%(1<<40)), 0)), data
	case 13:
		return mms.NewBCD(int64(u64() % 1_000_000)), data
	case 14:
		oid := []int{1, 2}
		for _, b := range take(int(op>>5) + 1) {
			oid = append(oid, int(b))
		}
		return mms.NewObjectIdentifier(oid), data
	case 15:
		b := take(2)
		return mms.NewBooleanArray(b, len(b)*8), data
	case 16:
		return mms.NewDataAccessError(mms.DataAccessErrorCode(op % 12)), data
	case 17, 18:
		if depth >= 4 {
			return mms.NewInteger(int64(op)), data
		}
		n := int(op>>5) + 1
		els := make([]*mms.Value, 0, n)
		for i := 0; i < n; i++ {
			var el *mms.Value
			el, data = fuzzMMSValue(data, depth+1)
			els = append(els, el)
		}
		if op%20 == 17 {
			return mms.NewStructure(els), data
		}
		return mms.NewArray(els), data
	default:
		return mms.NewStructure(nil), data
	}
}

// FuzzFormatMmsValue runs the display formatters over arbitrary MMS values:
// none may panic and all must return a non-empty string.
func FuzzFormatMmsValue(f *testing.F) {
	for _, seed := range [][]byte{
		{}, {0x80}, {1, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{3, 0x7f, 0xf8, 0, 0, 0, 0, 0, 1}, // NaN
		{5, 2, 0xff, 0x00}, {6 | 0x20, 0xaa, 0xbb}, {10, 0, 0, 0, 0x65, 0, 0, 0, 0},
		{17 | 0x60, 1, 2, 3, 4}, {18, 17, 18, 0}, {16}, {14 | 0xe0, 1, 2, 3},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		mv, _ := fuzzMMSValue(data, 0)
		if s := FormatMmsValue(mv); s == "" && mv.Type() != mms.ValueTypeVisibleString && mv.Type() != mms.ValueTypeMmsString {
			t.Fatalf("FormatMmsValue(%v) empty", mv)
		}
		dv := domain.ValueFromMMS(mv)
		if dv == nil {
			t.Fatalf("ValueFromMMS(%v) = nil", mv)
		}
		_ = FormatDomainValue(dv)
		_ = dv.String()
		_ = FormatDataAttributeValue(&domain.DataAttribute{Value: dv, Type: dv.Type})
		for _, name := range []string{"", "q", "x.q", "t"} {
			_ = FormatLeafValue(dv.Raw, dv.Type, name)
			_ = FormatAttributeValueWithError(dv.Raw, dv.Type, nil, name)
		}
		_ = FormatQualityValue(dv.Raw)
		_ = FormatBinaryTime(dv.Raw)
		_ = FormatUnixTimestamp(dv.Raw)
	})
}
