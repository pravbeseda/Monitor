package hub

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/pravbeseda/monitor/internal/i18n"
	"github.com/pravbeseda/monitor/internal/storage"
)

//go:embed templates/index.html
var templates embed.FS

var pageTemplate = template.Must(template.ParseFS(templates, "templates/index.html"))

// view is the page as the template sees it: every string is already translated and every
// number already formatted, so the template holds no logic and no English.
type view struct {
	Locale         i18n.Locale
	Title          string
	Empty          string
	NoValues       string
	LastSeenLabel  string
	MetricLabel    string
	VolumeLabel    string
	ValueLabel     string
	CollectedLabel string
	Nodes          []nodeView
}

type nodeView struct {
	Name     string
	LastSeen string
	Values   []valueView
}

type valueView struct {
	Metric    string
	Volume    string
	Value     string
	Collected string
}

// Page renders the latest state of every node.
func Page(store storage.Storage) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		printer := i18n.For(i18n.Negotiate(r.URL.Query().Get("lang"), r.Header.Get("Accept-Language")))

		states, err := store.States(r.Context())
		if err != nil {
			slog.Error("read node states", "error", err)
			http.Error(w, printer.T("error.storage"), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := pageTemplate.Execute(w, render(printer, states)); err != nil {
			slog.Error("render the page", "error", err)
		}
	})
}

func render(printer *i18n.Printer, states []storage.NodeState) view {
	out := view{
		Locale:         printer.Locale(),
		Title:          printer.T("page.title"),
		Empty:          printer.T("page.empty"),
		NoValues:       printer.T("node.no_values"),
		LastSeenLabel:  printer.T("node.last_seen"),
		MetricLabel:    printer.T("table.metric"),
		VolumeLabel:    printer.T("table.volume"),
		ValueLabel:     printer.T("table.free"),
		CollectedLabel: printer.T("table.collected"),
		Nodes:          make([]nodeView, 0, len(states)),
	}
	for _, state := range states {
		node := nodeView{
			Name:     state.Node,
			LastSeen: printer.Time(state.LastSeen),
			Values:   make([]valueView, 0, len(state.Values)),
		}
		for _, value := range state.Values {
			node.Values = append(node.Values, valueView{
				Metric:    value.Metric,
				Volume:    volume(printer, value.Labels),
				Value:     format(printer, value.Metric, value.Value),
				Collected: printer.Time(value.TS),
			})
		}
		out.Nodes = append(out.Nodes, node)
	}
	return out
}

// volume names the thing a series is about, from the labels the sensor set.
func volume(printer *i18n.Printer, labels map[string]string) string {
	parts := make([]string, 0, 3)
	if mount := labels["mount"]; mount != "" {
		parts = append(parts, mount)
	}
	if fs := labels["fs"]; fs != "" {
		parts = append(parts, fs)
	}
	if labels["removable"] == "true" {
		parts = append(parts, printer.T("label.removable"))
	}
	return strings.Join(parts, " · ")
}

// format reads the unit from the metric id, which is where the wire format keeps it.
func format(printer *i18n.Printer, metric string, value float64) string {
	switch {
	case strings.HasSuffix(metric, "_bytes"):
		return printer.Bytes(value)
	case strings.HasSuffix(metric, "_pct"):
		return printer.Percent(value)
	default:
		return printer.Number(value)
	}
}
