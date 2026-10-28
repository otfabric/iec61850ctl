// SPDX-License-Identifier: MIT

package svops

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/go-iec61850/sv"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// ParseKindValue parses raw as a value of the sample encoding k:
//   - BOOLEAN: true/false
//   - INT*, INT*U, ENUM: integers (range-checked when encoded)
//   - FLOAT32/FLOAT64: numbers
//   - Dbpos: intermediate, off, on, bad or 0-3
//   - Quality: good, invalid, questionable, test, joined with "+"
//     ("invalid+test"); a 32-bit word ("0x0802"); or 13 MSB-first binary
//     digits as in the MMS value JSON ("0000000000010")
//   - Timestamp: "now" or RFC 3339
func ParseKindValue(raw string, k sv.Kind) (*mms.Value, error) {
	s := strings.TrimSpace(raw)
	bad := func() error { return errs.Invalidf("value %q is not a valid %s", raw, k) }
	switch k {
	case sv.KindBoolean:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, bad()
		}
		return mms.NewBoolean(b), nil
	case sv.KindInt8, sv.KindInt16, sv.KindInt32, sv.KindInt64, sv.KindEnum:
		i, err := strconv.ParseInt(s, 0, k.Size()*8)
		if err != nil {
			return nil, bad()
		}
		return mms.NewInteger(i), nil
	case sv.KindUint8, sv.KindUint16, sv.KindUint24, sv.KindUint32:
		u, err := strconv.ParseUint(s, 0, k.Size()*8)
		if err != nil {
			return nil, bad()
		}
		return mms.NewUnsigned(u), nil
	case sv.KindFloat32, sv.KindFloat64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || (k == sv.KindFloat32 && !math.IsInf(f, 0) && math.Abs(f) > math.MaxFloat32) {
			return nil, bad()
		}
		return mms.NewFloat(f), nil
	case sv.KindDbpos:
		names := []string{"intermediate", "off", "on", "bad"}
		for i, n := range names {
			if strings.EqualFold(s, n) || s == strconv.Itoa(i) {
				return mms.NewInteger(int64(i)), nil
			}
		}
		return nil, bad()
	case sv.KindQuality:
		q, err := ParseQuality(s)
		if err != nil {
			return nil, err
		}
		return q.Value(), nil
	case sv.KindTimestamp:
		if strings.EqualFold(s, "now") {
			return mms.NewUTCTime(time.Now().UTC()), nil
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, bad()
		}
		return mms.NewUTCTime(t.UTC()), nil
	case sv.KindInvalid:
	}
	return nil, errs.Invalidf("member type %s cannot be set", k)
}

// ParseQuality parses a quality: names joined with "+" (good, invalid,
// questionable, test, derived, old-data, substituted, failure,
// operator-blocked), a 32-bit word in hex or decimal ("0x0802"), or 13
// MSB-first binary digits ("0000000000010" = test).
func ParseQuality(s string) (sv.Quality, error) {
	s = strings.TrimSpace(s)
	if len(s) == 13 && strings.Trim(s, "01") == "" {
		var q sv.Quality
		for i, c := range s {
			if c == '1' {
				q |= 1 << i
			}
		}
		return q, nil
	}
	if n, err := strconv.ParseUint(s, 0, 32); err == nil {
		return sv.Quality(n), nil
	}
	names := map[string]sv.Quality{
		"good": sv.QualityGood, "invalid": sv.QualityInvalid, "questionable": sv.QualityQuestionable,
		"test": sv.QualityTest, "derived": sv.QualityDerived, "old-data": sv.QualityOldData,
		"substituted": sv.QualitySubstituted, "failure": sv.QualityFailure,
		"operator-blocked": sv.QualityOperatorBlocked,
	}
	var q sv.Quality
	for _, part := range strings.Split(s, "+") {
		bit, ok := names[strings.ToLower(strings.TrimSpace(part))]
		if !ok {
			return 0, errs.Invalidf("invalid quality %q (good, invalid, questionable, test, ... joined with +; 0xNNNN; or 13 binary digits)", s)
		}
		q |= bit
	}
	return q, nil
}

// ZeroValues returns the zero value of every leaf of layout (good quality,
// the epoch for timestamps).
func ZeroValues(layout sv.Layout) []*mms.Value {
	out := make([]*mms.Value, len(layout))
	for i, k := range layout {
		switch k {
		case sv.KindBoolean:
			out[i] = mms.NewBoolean(false)
		case sv.KindUint8, sv.KindUint16, sv.KindUint24, sv.KindUint32:
			out[i] = mms.NewUnsigned(0)
		case sv.KindFloat32, sv.KindFloat64:
			out[i] = mms.NewFloat(0)
		case sv.KindQuality:
			out[i] = sv.QualityGood.Value()
		case sv.KindTimestamp:
			out[i] = mms.NewUTCTime(time.Unix(0, 0).UTC())
		default:
			out[i] = mms.NewInteger(0)
		}
	}
	return out
}

// LeafNames returns the names a --value can address each leaf of cb's
// layout by, besides its index: the member reference for single-leaf
// members, REF#N for leaf N of a structured member, and for the 9-2LE
// dataset the channel names (IA ... VN) and their qualities (qIA ... qVN).
func LeafNames(cb *sv.ControlBlock) [][]string {
	out := make([][]string, len(cb.Layout))
	i := 0
	for _, m := range cb.Members {
		for j := range m.Kinds {
			if i >= len(out) {
				break
			}
			if len(m.Kinds) == 1 {
				out[i] = append(out[i], m.Ref)
			}
			out[i] = append(out[i], fmt.Sprintf("%s#%d", m.Ref, j))
			i++
		}
	}
	if IsLE(cb.Layout) {
		for c, name := range LEChannels {
			out[2*c] = append(out[2*c], name)
			out[2*c+1] = append(out[2*c+1], "q"+name)
		}
	}
	return out
}

// ApplyOverrides sets leaves of values from specs of the form INDEX=VALUE
// or NAME=VALUE (see LeafNames).
func ApplyOverrides(cb *sv.ControlBlock, values []*mms.Value, specs []string) error {
	names := LeafNames(cb)
	for _, spec := range specs {
		key, raw, ok := strings.Cut(spec, "=")
		if !ok {
			return errs.Invalidf("--value %q: want INDEX=VALUE or NAME=VALUE", spec)
		}
		key = strings.TrimSpace(key)
		idx := -1
		if n, err := strconv.Atoi(key); err == nil {
			idx = n
		} else {
			for i, ns := range names {
				for _, name := range ns {
					if name == key {
						idx = i
					}
				}
			}
		}
		if idx < 0 || idx >= len(values) {
			return errs.Invalidf("--value %q: no leaf %q (0-%d, a member reference or a 9-2LE channel)", spec, key, len(values)-1)
		}
		v, err := ParseKindValue(raw, cb.Layout[idx])
		if err != nil {
			return errs.Mark(fmt.Errorf("--value %s: %w", key, err), errs.ErrInvalidInput)
		}
		values[idx] = v
	}
	return nil
}

// Waveform is a sine on one 9-2LE channel: Amplitude is the peak value in
// amperes (IA-IN) or volts (VA-VN).
type Waveform struct {
	Channel   int // 0-7 in LEChannels order
	Amplitude float64
	Frequency float64 // Hz
	Phase     float64 // degrees
}

// ParseWaveform parses CHANNEL=AMPLITUDE[,FREQUENCY[,PHASE]] with the
// frequency in Hz (default nominalHz) and the phase in degrees (default 0),
// e.g. "IA=100", "VB=325,50,-120".
func ParseWaveform(spec string, nominalHz float64) (Waveform, error) {
	bad := func(why string) error {
		return errs.Invalidf("--waveform %q: %s (want CHANNEL=AMPLITUDE[,FREQUENCY[,PHASE]], channel IA..IN or VA..VN)", spec, why)
	}
	name, params, ok := strings.Cut(spec, "=")
	if !ok {
		return Waveform{}, bad("missing '='")
	}
	w := Waveform{Channel: -1, Frequency: nominalHz}
	for i, ch := range LEChannels {
		if strings.EqualFold(strings.TrimSpace(name), ch) {
			w.Channel = i
		}
	}
	if w.Channel < 0 {
		return Waveform{}, bad("unknown channel")
	}
	parts := strings.Split(params, ",")
	if len(parts) > 3 {
		return Waveform{}, bad("too many parameters")
	}
	nums := make([]float64, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return Waveform{}, bad("not a number: " + strings.TrimSpace(p))
		}
		nums[i] = f
	}
	w.Amplitude = nums[0]
	if len(nums) > 1 {
		w.Frequency = nums[1]
	}
	if len(nums) > 2 {
		w.Phase = nums[2]
	}
	if w.Frequency < 0 {
		return Waveform{}, bad("negative frequency")
	}
	return w, nil
}

// WaveformSource returns a 9-2LE sample source: base gives the static
// value and quality of every channel (16 leaves in LELayout order) and
// each waveform replaces one channel's value with a sine sampled at
// samplesPerSecond. Time is the sample index divided by the rate, so the
// signal is exact regardless of scheduling jitter.
func WaveformSource(base []*mms.Value, waves []Waveform, samplesPerSecond float64) (func(sv.SampleInfo, []byte) []byte, error) {
	if len(base) != len(sv.LELayout) {
		return nil, errs.Invalidf("--waveform needs the 9-2LE dataset (16 leaves), got %d", len(base))
	}
	if samplesPerSecond <= 0 {
		return nil, errs.Invalidf("--waveform needs a positive sample rate")
	}
	var static sv.LESample
	for c := range LEChannels {
		v, _ := base[2*c].Int64()
		q, err := sv.QualityFromValue(base[2*c+1])
		if err != nil {
			return nil, errs.Mark(fmt.Errorf("quality of %s: %w", LEChannels[c], err), errs.ErrInvalidInput)
		}
		p := sv.Point{Value: int32(max(min(v, math.MaxInt32), math.MinInt32)), Q: q}
		if c < 4 {
			static.Current[c] = p
		} else {
			static.Voltage[c-4] = p
		}
	}
	waves = append([]Waveform(nil), waves...)
	return func(info sv.SampleInfo, dst []byte) []byte {
		s := static
		t := float64(info.Index) / samplesPerSecond
		for _, w := range waves {
			x := w.Amplitude * math.Sin(2*math.Pi*w.Frequency*t+w.Phase*math.Pi/180)
			if w.Channel < 4 {
				s.Current[w.Channel].Value = sv.AmpsCount(x)
			} else {
				s.Voltage[w.Channel-4].Value = sv.VoltsCount(x)
			}
		}
		return s.Append(dst)
	}, nil
}
