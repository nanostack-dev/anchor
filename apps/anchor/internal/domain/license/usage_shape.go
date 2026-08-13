package license

import "slices"

// UsageShape declares whether a limit's usage is a gauge or a windowed
// counter — see [ReportUsageInput] and docs/adr/0003-usage-reported-as-snapshots.md
// for what each means.
//
// It is declared once, on the schema field, and checked against every report
// against that field, rather than inferred per report. A per-report choice
// would let the same key's series silently mix an accumulating count with a
// point-in-time reading — nothing would catch a caller reporting a windowed
// counter for a month, then a gauge the next day by omitting the window, and
// anything that differences the series afterward (which
// docs/adr/0003-usage-reported-as-snapshots.md calls safe) would compute
// against a value that means something categorically different. See
// docs/adr/0012-usage-shape-is-declared-not-inferred.md.
type UsageShape string

const (
	// UsageShapeGauge is a limit whose usage rises and falls: "this many exist
	// right now." A report against it must not carry a window.
	UsageShapeGauge UsageShape = "GAUGE"
	// UsageShapeWindowedCounter is a limit whose usage accumulates within a
	// business-defined period and resets when a new one starts. A report
	// against it must carry a window.
	UsageShapeWindowedCounter UsageShape = "WINDOWED_COUNTER"
)

// UsageShapes lists every recognised usage shape, for validation and for
// enumerating the type in the API contract.
func UsageShapes() []UsageShape {
	return []UsageShape{UsageShapeGauge, UsageShapeWindowedCounter}
}

// Valid reports whether s is one of the recognised shapes.
func (s UsageShape) Valid() bool {
	return slices.Contains(UsageShapes(), s)
}
