// SPDX-License-Identifier: MIT

package formatter

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// FuzzScalarJSONValue checks that the JSON projection of any decoded MMS
// leaf value is JSON-encodable (a failing Marshal silently drops jsonl
// report events and breaks get object --format json).
func FuzzScalarJSONValue(f *testing.F) {
	f.Add(uint8(3), uint64(math.Float64bits(math.NaN())), "")
	f.Add(uint8(3), uint64(math.Float64bits(math.Inf(1))), "")
	f.Add(uint8(4), uint64(math.Float64bits(math.Inf(-1))), "")
	f.Add(uint8(1), uint64(1<<63), "")
	f.Add(uint8(7), uint64(0), "\xff\xfe")
	f.Add(uint8(9), uint64(1_700_000_000), "")
	f.Fuzz(func(t *testing.T, op uint8, n uint64, s string) {
		var mv *mms.Value
		switch op % 12 {
		case 0:
			mv = mms.NewBoolean(n&1 == 1)
		case 1:
			mv = mms.NewInteger(int64(n))
		case 2:
			mv = mms.NewUnsigned(n)
		case 3:
			mv = mms.NewFloat(math.Float64frombits(n))
		case 4:
			mv = mms.NewReal(math.Float64frombits(n))
		case 5:
			mv = mms.NewVisibleString(s)
		case 6:
			mv = mms.NewMmsString(s)
		case 7:
			mv = mms.NewOctetString([]byte(s))
		case 8:
			mv = mms.NewBitString([]byte(s))
		case 9:
			mv = mms.NewUTCTime(time.Unix(int64(n%(1<<34)), 0))
		case 10:
			mv = mms.NewBinaryTime(int64(n))
		default:
			mv = mms.NewStructure([]*mms.Value{mms.NewFloat(math.Float64frombits(n)), mms.NewVisibleString(s)})
		}
		v, err := ScalarJSONValue(domain.ValueFromMMS(mv))
		if err != nil {
			return
		}
		if _, err := json.Marshal(v); err != nil {
			t.Fatalf("json.Marshal(ScalarJSONValue(%v)) = %v", mv, err)
		}
		_ = JSONTypeName(domain.FromMMSValueType(mv.Type()))
	})
}

// FuzzParseFormats fuzzes the --format flag parsers.
func FuzzParseFormats(f *testing.F) {
	for _, s := range []string{"", "text", "JSON", " yaml ", "csv", "table", "jsonl", "ndjson", "xml"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if of, err := ParseCLIFormat(s); err == nil && of != OutputFormatText && of != OutputFormatJSON {
			t.Fatalf("ParseCLIFormat(%q) = %v", s, of)
		}
		if sf, err := ParseStreamFormat(s); err == nil {
			if again, err := ParseStreamFormat(string(sf)); err != nil || again != sf {
				t.Fatalf("ParseStreamFormat round trip %q -> %q: %q %v", s, sf, again, err)
			}
		}
	})
}
