- internal/rules/window.go: per-host deque of recent events with GC.
  API:
    Add(ev Event)
    RecentByType(hostID string, eventType string, within time.Duration) []Event
    GC(now time.Time)
- Unit tests: time-window filter & GC.
