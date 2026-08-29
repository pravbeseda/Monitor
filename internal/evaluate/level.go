package evaluate

// Level is how bad a subject is right now. The order matters: hysteresis holds a subject
// at a level while `previous >= level`.
type Level int

// The three levels a subject can hold, least severe first.
const (
	OK Level = iota
	Warning
	Critical
)

var levelNames = [...]string{OK: "ok", Warning: "warning", Critical: "critical"}

// String is the level's stored and logged form; storage keeps a level as this string
// rather than as a number, so a reordering here cannot rewrite history.
func (l Level) String() string {
	if l < OK || int(l) >= len(levelNames) {
		return "unknown"
	}
	return levelNames[l]
}
