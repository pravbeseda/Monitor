package evaluate

// Product defaults from ADR 0012: how the tool judges a volume out of the box. They
// describe behaviour, not any installation, so they live in code and every layer of the
// hub's configuration overrides them (ADR 0007 rule 1).

// defaultDiskRule is a floor that catches any volume close to full, and a band that
// catches one running low proportionally while its headroom is small enough to matter.
func defaultDiskRule() Rule {
	return Rule{
		Warning:  Threshold{Floor: 10e9, Ratio: 15, Ceiling: 100e9},
		Critical: Threshold{Floor: 4e9, Ratio: 7, Ceiling: 40e9},
	}
}

// defaultBackupRule drops the band: a backup volume sits nearly full by design, so
// percentages say nothing about it and only absolute headroom counts.
func defaultBackupRule() Rule {
	return Rule{
		Warning:  Threshold{Floor: 50e9},
		Critical: Threshold{Floor: 10e9},
	}
}
