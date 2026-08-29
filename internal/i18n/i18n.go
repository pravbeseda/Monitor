// Package i18n holds every user-facing string and the locale rules that go with them.
// No user-facing text is written at its usage site (ADR 0008); log lines and CLI output
// stay English and never come from here.
package i18n

import "strings"

// Locale is one of the two languages the interface speaks.
type Locale string

// The two languages of ADR 0008; English is also the fallback.
const (
	English Locale = "en"
	Russian Locale = "ru"
)

// Negotiate picks the locale for one request: an explicit ?lang= wins, then the
// browser's Accept-Language, then English.
func Negotiate(query, acceptLanguage string) Locale {
	if locale, ok := match(query); ok {
		return locale
	}
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		if locale, ok := match(tag); ok {
			return locale
		}
	}
	return English
}

// match accepts a regional tag too: ru-BY is Russian.
func match(tag string) (Locale, bool) {
	language, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	switch Locale(language) {
	case English:
		return English, true
	case Russian:
		return Russian, true
	default:
		return "", false
	}
}

// Printer renders text and numbers in one locale.
type Printer struct {
	locale Locale
}

// For returns the printer of a locale.
func For(locale Locale) *Printer { return &Printer{locale: locale} }

// Locale is the language this printer speaks.
func (p *Printer) Locale() Locale { return p.locale }

// T looks a string up by its English identifier. An unknown key renders as itself: a
// missing translation is a bug, and an empty cell hides it.
func (p *Printer) T(key string) string {
	texts, known := catalogue[key]
	if !known {
		return key
	}
	if text, translated := texts[p.locale]; translated {
		return text
	}
	return texts[English]
}
