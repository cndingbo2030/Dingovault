package mobile

import (
	"encoding/json"
	"sync"
)

var evMu sync.RWMutex
var evSink EventSink

// SetEventSink registers the native bridge used to deliver events to JavaScript (WebView).
// Pass nil to detach.
func SetEventSink(s EventSink) {
	evMu.Lock()
	evSink = s
	evMu.Unlock()
}

func emitToSink(name string, payload map[string]any) {
	evMu.RLock()
	s := evSink
	evMu.RUnlock()
	if s == nil || name == "" {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.Emit(name, string(b))
}
