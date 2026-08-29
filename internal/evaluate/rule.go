package evaluate

import "math"

// A rule enters a level when the volume is short of space in absolute terms, or short of
// it proportionally while its absolute headroom is still small enough to matter
// (ADR 0012). It leaves that level when the whole rule stops holding, with a 20% margin on
// every comparison (ADR 0013).
const (
	marginNumerator   = 6
	marginDenominator = 5
)

// Threshold is one level of a rule: a floor, optionally guarded by a proportional band.
// A rule for a backup volume has no band, so only the floor comparison remains.
type Threshold struct {
	// Floor is absolute headroom, in bytes.
	Floor float64
	// Ratio is percentage points of the volume; Ceiling is the absolute headroom above
	// which the ratio stops mattering. A band needs both, and the hub refuses a
	// configuration that declares one without the other.
	Ratio   float64
	Ceiling float64
}

func (t Threshold) banded() bool { return t.Ratio > 0 && t.Ceiling > 0 }

// entered reports whether a subject at these values falls into this level.
func (t Threshold) entered(free, pct float64) bool {
	return free < t.Floor || (t.banded() && pct < t.Ratio && free < t.Ceiling)
}

// left is the negation of entered with the margin on every comparison. An absent band
// contributes false to the entry disjunction and true to this conjunction, so neither side
// degenerates: true at entry would alert every volume, false here would latch every one.
func (t Threshold) left(free, pct float64) bool {
	return clearedBytes(free, t.Floor) &&
		(!t.banded() || clearedPercent(pct, t.Ratio) || clearedBytes(free, t.Ceiling))
}

// The margin is applied as 5×value against 6×threshold rather than as value against
// 1.2×threshold, because 1.2 has no exact binary form and a value sitting exactly on its
// margin must count as cleared. Byte counts are whole numbers, so both products are exact
// for any volume the project will meet; percentages arrive with two decimals
// (docs/specs/disk-sensor.md), so they are taken in hundredths first.
func clearedBytes(value, threshold float64) bool {
	return value*marginDenominator >= threshold*marginNumerator
}

func clearedPercent(value, threshold float64) bool {
	return math.Round(value*100)*marginDenominator >= math.Round(threshold*100)*marginNumerator
}

// Rule is what gives one subject its level: the thresholds of both levels.
type Rule struct {
	Warning  Threshold
	Critical Threshold
}

// Level is the level a subject reaches from previous at these values. Levels are tried
// most severe first: a level is entered when its rule holds, and held when the subject was
// already at least that severe and its rule has not been left.
func (r Rule) Level(previous Level, free, pct float64) Level {
	for _, level := range []Level{Critical, Warning} {
		threshold := r.threshold(level)
		if threshold.entered(free, pct) {
			return level
		}
		if previous >= level && !threshold.left(free, pct) {
			return level
		}
	}
	return OK
}

func (r Rule) threshold(level Level) Threshold {
	if level == Critical {
		return r.Critical
	}
	return r.Warning
}
