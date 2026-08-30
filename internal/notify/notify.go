// Package notify delivers what evaluation decides to say. A channel formats and sends; it
// never decides what is worth sending (docs/specs/evaluation.md#messages).
package notify

import (
	"fmt"
	"strings"

	"github.com/pravbeseda/monitor/internal/evaluate"
	"github.com/pravbeseda/monitor/internal/i18n"
)

// levelKeys name the catalogue entry of each level, so a level's stored name and its
// user-facing text can never be the same string by accident.
var levelKeys = map[evaluate.Level]string{
	evaluate.OK:       "level.ok",
	evaluate.Warning:  "level.warning",
	evaluate.Critical: "level.critical",
}

// Render is the one text every channel sends. A message whose levels are equal is a repeat
// or a digest entry for a subject that has not moved, and reads as a standing level rather
// than as a change.
func Render(p *i18n.Printer, m evaluate.Message) string {
	subject := m.Node
	if mount := m.Labels["mount"]; mount != "" {
		subject += " " + mount
	}

	line := fmt.Sprintf(p.T("notify.changed"),
		subject, p.T(levelKeys[m.To]), p.T(levelKeys[m.From]), p.Time(m.Since))
	if m.From == m.To {
		line = fmt.Sprintf(p.T("notify.standing"), subject, p.T(levelKeys[m.To]), p.Time(m.Since))
	}
	if detail := detail(p, m); detail != "" {
		line += " — " + detail
	}
	return line
}

// RenderDigest is the day's summary as one message: a title and one line per subject.
func RenderDigest(p *i18n.Printer, entries []evaluate.Message) string {
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, p.T("digest.title"))
	for _, entry := range entries {
		lines = append(lines, Render(p, entry))
	}
	return strings.Join(lines, "\n")
}

// detail renders the values that produced a message. A rule reads two series, so what is
// shown comes from its definition rather than from the metric ids written out here; the
// silence subject has no values at all and says so.
func detail(p *i18n.Printer, m evaluate.Message) string {
	if m.Rule == evaluate.SilenceRule {
		return p.T("notify.silent")
	}
	definition, known := evaluate.Lookup(m.Rule)
	if !known {
		return ""
	}
	free, gotFree := m.Readings[definition.Free]
	pct, gotPct := m.Readings[definition.Pct]
	if !gotFree || !gotPct {
		return ""
	}
	return fmt.Sprintf(p.T("notify.readings"), p.Bytes(free), p.Percent(pct))
}
