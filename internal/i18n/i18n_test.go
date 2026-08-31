package i18n_test

import (
	"testing"
	"time"

	"github.com/pravbeseda/monitor/internal/i18n"
)

func TestNegotiate(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		header string
		want   i18n.Locale
	}{
		{"the query wins over the browser", "en", "ru-RU,ru;q=0.9", i18n.English},
		{"the browser is followed when the query is silent", "", "ru-RU,ru;q=0.9,en;q=0.8", i18n.Russian},
		{"nothing asked for means English", "", "", i18n.English},
		{"an unsupported language falls back to English", "de", "de-DE", i18n.English},
		{"an unsupported query leaves the choice to the browser", "de", "ru-RU", i18n.Russian},
		{"a regional tag still matches its language", "", "ru-BY", i18n.Russian},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := i18n.Negotiate(tc.query, tc.header); got != tc.want {
				t.Errorf("Negotiate(%q, %q) = %q, want %q", tc.query, tc.header, got, tc.want)
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	if got := i18n.For(i18n.English).T("page.title"); got != "Monitor" {
		t.Errorf("English title = %q", got)
	}
	if got := i18n.For(i18n.Russian).T("table.free"); got == "" || got == "table.free" {
		t.Errorf("Russian text for table.free = %q, want a translation", got)
	}
}

// A key with no text is a bug in the catalogue, so it shows as itself rather than as an
// empty cell that nobody notices.
func TestTranslateShowsAnUnknownKey(t *testing.T) {
	if got := i18n.For(i18n.English).T("nothing.here"); got != "nothing.here" {
		t.Errorf("unknown key rendered as %q, want the key itself", got)
	}
}

func TestFormatsFollowTheLocale(t *testing.T) {
	at := time.Date(2026, 8, 28, 10, 5, 0, 0, time.UTC)
	tests := []struct {
		locale               i18n.Locale
		bytes, percent, when string
	}{
		{i18n.English, "1.5 GB", "34.2%", "2026-08-28 10:05 UTC"},
		{i18n.Russian, "1,5 ГБ", "34,2 %", "28.08.2026 10:05 UTC"},
	}

	for _, tc := range tests {
		t.Run(string(tc.locale), func(t *testing.T) {
			p := i18n.For(tc.locale)
			if got := p.Bytes(1.5e9); got != tc.bytes {
				t.Errorf("Bytes = %q, want %q", got, tc.bytes)
			}
			if got := p.Percent(34.24); got != tc.percent {
				t.Errorf("Percent = %q, want %q", got, tc.percent)
			}
			if got := p.Time(at); got != tc.when {
				t.Errorf("Time = %q, want %q", got, tc.when)
			}
		})
	}
}

func TestBytesScalesToTheValue(t *testing.T) {
	p := i18n.For(i18n.English)
	tests := map[float64]string{
		512:    "512 B",
		2048:   "2.0 kB",
		3.3e6:  "3.3 MB",
		2.5e12: "2.5 TB",
	}

	for value, want := range tests {
		if got := p.Bytes(value); got != want {
			t.Errorf("Bytes(%v) = %q, want %q", value, got, want)
		}
	}
}

// Parse reads a locale written by hand, where a regional tag is a mistake rather than a
// browser's habit.
func TestParse(t *testing.T) {
	spoken := []struct {
		tag  string
		want i18n.Locale
	}{
		{"en", i18n.English},
		{"EN", i18n.English},
		{"  ru  ", i18n.Russian},
	}
	for _, tc := range spoken {
		if got, ok := i18n.Parse(tc.tag); !ok || got != tc.want {
			t.Fatalf("Parse(%q) = %q, %v; want %q, true", tc.tag, got, ok, tc.want)
		}
	}
	for _, tag := range []string{"ru-BY", "", "english", "fr"} {
		if got, ok := i18n.Parse(tag); ok {
			t.Fatalf("Parse(%q) = %q, true; want it refused", tag, got)
		}
	}
}

// A locale the catalogue does not know reaches a printer through configuration or a stale
// link; it renders English rather than failing at the one call that has no English path.
func TestPrinterFallsBackToEnglish(t *testing.T) {
	p := i18n.For(i18n.Locale("de"))
	if got := p.Locale(); got != i18n.English {
		t.Errorf("Locale = %q, want %q", got, i18n.English)
	}
	if got := p.Bytes(1.5e9); got != "1.5 GB" {
		t.Errorf("Bytes = %q, want the English form", got)
	}
	if got := p.Percent(34.24); got != "34.2%" {
		t.Errorf("Percent = %q, want the English form", got)
	}
	if got := p.Time(time.Date(2026, 8, 28, 10, 5, 0, 0, time.UTC)); got != "2026-08-28 10:05 UTC" {
		t.Errorf("Time = %q, want the English form", got)
	}
	if got := p.T("page.title"); got != "Monitor" {
		t.Errorf("T = %q, want the English text", got)
	}
}
