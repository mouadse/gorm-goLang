package api

import (
	"encoding/json"
	"log"
	"sync"
)

// AdminRealtimeEvent represents a push event that should be broadcast
// to all connected admin WebSocket clients immediately.
type AdminRealtimeEvent struct {
	Type    string         `json:"type"` // e.g. "user_signup", "metrics_update"
	Payload map[string]any `json:"payload"`
}

// AdminRealtimeHub maintains a set of active admin WebSocket connections
// and broadcasts events to all of them.
type AdminRealtimeHub struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

func NewAdminRealtimeHub() *AdminRealtimeHub {
	return &AdminRealtimeHub{
		subscribers: make(map[chan []byte]struct{}),
	}
}

// Subscribe registers a channel and returns it. The caller should read
// from this channel and write the bytes to the WebSocket connection.
func (h *AdminRealtimeHub) Subscribe() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel and closes it.
func (h *AdminRealtimeHub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	if _, ok := h.subscribers[ch]; ok {
		delete(h.subscribers, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// Broadcast sends an event to all connected admin WebSocket clients.
func (h *AdminRealtimeHub) Broadcast(event AdminRealtimeEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("admin-realtime-hub: marshal event: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subscribers {
		select {
		case ch <- data:
		default:
			// Channel full — drop the message so we don't block the broadcaster.
			log.Printf("admin-realtime-hub: subscriber channel full, dropping event")
		}
	}
}

// NotifyUserSignup is a convenience method to push an immediate
// user-signup event to all connected admin dashboards.
func (h *AdminRealtimeHub) NotifyUserSignup(totalUsers int64, newUsers7d int64) {
	h.Broadcast(AdminRealtimeEvent{
		Type: "user_signup",
		Payload: map[string]any{
			"total_users":  totalUsers,
			"new_users_7d": newUsers7d,
		},
	})
}
