package gmail

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"sync"

	"google.golang.org/api/gmail/v1"
)

type PushHandler struct {
	GmailService *gmail.Service
	DB           interface{} // TODO: Replace with actual DB type
	AIResponder  interface{} // TODO: Replace with actual AI responder type
	lastHistoryID string
	lock         sync.Mutex
}

func (h *PushHandler) HandlePushNotification(data map[string]interface{}) {
	h.lock.Lock()
	defer h.lock.Unlock()
	log.Println("HandlePushNotification called")

	// Decode Pub/Sub message
	msg, ok := data["message"].(map[string]interface{})
	if !ok {
		log.Println("Invalid notification: missing 'message'")
		return
	}
	encoded, ok := msg["data"].(string)
	if !ok {
		log.Println("Invalid notification: missing 'data'")
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Printf("Failed to decode base64: %v", err)
		return
	}
	var gmailNotification map[string]interface{}
	if err := json.Unmarshal(decoded, &gmailNotification); err != nil {
		log.Printf("Failed to unmarshal notification: %v", err)
		return
	}
	emailAddress, _ := gmailNotification["emailAddress"].(string)
	historyID, _ := gmailNotification["historyId"].(string)
	log.Printf("Push notification for %s, history ID: %s", emailAddress, historyID)

	h.processHistoryChanges(historyID)
}

func (h *PushHandler) processHistoryChanges(currentHistoryID string) {
	if h.lastHistoryID == "" {
		log.Println("No previous history ID, skipping history processing")
		h.lastHistoryID = currentHistoryID
		return
	}
	log.Printf("Processing history from %s to %s", h.lastHistoryID, currentHistoryID)

	// TODO: Use GmailService to fetch history records since lastHistoryID
	// For each messageAdded, call h.processNewMessage(messageID, threadID)
	// After processing, update lastHistoryID

	// Example stub:
	log.Println("[STUB] Would fetch Gmail history and process new messages here")
	h.lastHistoryID = currentHistoryID
}

// processNewMessage would process a new message (stub for now)
func (h *PushHandler) processNewMessage(messageID, threadID string) {
	log.Printf("[STUB] Would process new message %s in thread %s", messageID, threadID)
} 