package i18n

// catalogue is keyed by English identifiers, so a missing translation is visible rather
// than silently English.
var catalogue = map[string]map[Locale]string{
	"page.title":      {English: "Monitor", Russian: "Монитор"},
	"page.nodes":      {English: "Nodes", Russian: "Узлы"},
	"page.empty":      {English: "No node has reported yet", Russian: "Ни один узел ещё не отчитался"},
	"node.last_seen":  {English: "Last seen", Russian: "Последний отчёт"},
	"node.no_values":  {English: "No measurements yet", Russian: "Измерений ещё нет"},
	"table.metric":    {English: "Metric", Russian: "Метрика"},
	"table.volume":    {English: "Volume", Russian: "Том"},
	"table.free":      {English: "Value", Russian: "Значение"},
	"table.collected": {English: "Collected", Russian: "Собрано"},
	"label.removable": {English: "removable", Russian: "съёмный"},
	"error.storage":   {English: "The panel cannot read its data right now", Russian: "Панель сейчас не может прочитать свои данные"},
}
