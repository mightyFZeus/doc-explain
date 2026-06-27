package service

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mightyfzeus/doc-explain/internal/dtos"
)

type DocumentStatusHub struct {
	mu      sync.RWMutex
	clients map[string]map[*websocket.Conn]bool
}

func NewDocumentStatusHub() *DocumentStatusHub {
	return &DocumentStatusHub{
		clients: make(map[string]map[*websocket.Conn]bool),
	}
}

func (h *DocumentStatusHub) Add(documentId string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[documentId] == nil {
		h.clients[documentId] = make(map[*websocket.Conn]bool)
	}

	h.clients[documentId][conn] = true
}

func (h *DocumentStatusHub) Delete(documentId string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[documentId] != nil {
		delete(h.clients[documentId], conn)
	}
}

func (h *DocumentStatusHub) Broadcast(event dtos.DocumentStatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.clients[event.DocumentID] {
		_ = conn.WriteJSON(event)
	}
}
