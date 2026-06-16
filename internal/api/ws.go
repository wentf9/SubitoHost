package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSNoteMsg struct {
	Type   string `json:"type"`   // "note_on" or "note_off"
	Key    int    `json:"key"`    // 0-127
	Vel    int    `json:"vel"`    // 0-127
	Source string `json:"source"` // "input" or "output"
}

type WSNoteAction struct {
	Action string `json:"action"` // "note_on" or "note_off"
	Key    int    `json:"key"`
	Vel    int    `json:"vel"`
	Target string `json:"target"` // "input" or "output"
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	ch := s.engine.Subscribe()
	defer s.engine.Unsubscribe(ch)

	// Read pump (drains client messages to detect disconnects and processes incoming events)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg WSNoteAction
			if err := json.Unmarshal(data, &msg); err == nil {
				if msg.Action == "note_on" || msg.Action == "note_off" {
					s.engine.InjectNote(msg.Action, msg.Key, msg.Vel, msg.Target)
				}
			}
		}
	}()

	for ev := range ch {
		var msg interface{} = ev

		if ev.Type == "note_on" || ev.Type == "note_off" {
			if noteData, ok := ev.Data.(map[string]interface{}); ok {
				msg = WSNoteMsg{
					Type:   ev.Type,
					Key:    noteData["key"].(int),
					Vel:    noteData["vel"].(int),
					Source: noteData["source"].(string),
				}
			}
		}

		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			break
		}
	}
}
