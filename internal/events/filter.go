package events

// FilterFunc returns true if the event should be passed through.
type FilterFunc func(event Event) bool

// FilteredHandler wraps a handler with a filter.
// The handler only receives events that pass the filter.
func FilteredHandler(filter FilterFunc, handler HandlerFunc) HandlerFunc {
	return func(event Event) {
		if filter(event) {
			handler(event)
		}
	}
}

// NodeFilter returns a filter that only passes events for a specific node.
func NodeFilter(nodeID string) FilterFunc {
	return func(event Event) bool {
		switch e := event.(type) {
		case NodeStarted:
			return e.NodeID == nodeID
		case NodeCompleted:
			return e.NodeID == nodeID
		case NodeError:
			return e.NodeID == nodeID
		case BatchStarted:
			return e.NodeID == nodeID
		case BatchCompleted:
			return e.NodeID == nodeID
		default:
			return false
		}
	}
}

// SeverityFilter returns a filter that only passes findings of a minimum severity.
func SeverityFilter(minSeverity string) FilterFunc {
	levels := map[string]int{
		"info": 0, "low": 1, "medium": 2, "high": 3, "critical": 4,
	}
	minLevel := levels[minSeverity]

	return func(event Event) bool {
		if e, ok := event.(FindingDiscovered); ok {
			return levels[e.Severity] >= minLevel
		}
		return true // pass non-finding events through
	}
}
