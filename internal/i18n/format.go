package i18n

import (
	"fmt"
	"strings"
	"time"
)

// Byte sizes use decimal units, the way disks are sold and the way both macOS and the
// hosting panels report them.
var byteUnits = map[Locale][]string{
	English: {"B", "kB", "MB", "GB", "TB", "PB"},
	Russian: {"Б", "кБ", "МБ", "ГБ", "ТБ", "ПБ"},
}

var timeLayouts = map[Locale]string{
	English: "2006-01-02 15:04 MST",
	Russian: "02.01.2006 15:04 MST",
}

// Bytes renders a size in the largest unit that keeps it above one.
func (p *Printer) Bytes(value float64) string {
	units := byteUnits[p.locale]
	size, unit := value, units[0]
	for _, next := range units[1:] {
		if size < 1000 {
			break
		}
		size, unit = size/1000, next
	}
	if unit == units[0] {
		return fmt.Sprintf("%s %s", p.number(size, 0), unit)
	}
	return fmt.Sprintf("%s %s", p.number(size, 1), unit)
}

// Percent renders a percentage the way each language writes one: 34.2% but 34,2 %.
func (p *Printer) Percent(value float64) string {
	if p.locale == Russian {
		return p.number(value, 1) + " %"
	}
	return p.number(value, 1) + "%"
}

// Time renders an instant in UTC: the hub stores UTC and the reader is one person.
func (p *Printer) Time(at time.Time) string {
	layout, known := timeLayouts[p.locale]
	if !known {
		layout = timeLayouts[English]
	}
	return at.UTC().Format(layout)
}

// Number renders a plain value: a metric whose id carries no unit still has to be shown.
func (p *Printer) Number(value float64) string { return p.number(value, 2) }

// number applies the decimal separator of the locale.
func (p *Printer) number(value float64, decimals int) string {
	text := fmt.Sprintf("%.*f", decimals, value)
	if p.locale == Russian {
		return strings.Replace(text, ".", ",", 1)
	}
	return text
}
