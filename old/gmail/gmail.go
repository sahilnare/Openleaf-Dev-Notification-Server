package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailService is a global Gmail API client
var GmailService *gmail.Service

// InitGmailService initializes the Gmail API client using credentials from file
func InitGmailService() error {
	ctx := context.Background()
	credFile := os.Getenv("GMAIL_TOKEN_FILE")
	if credFile == "" {
		credFile = "server_token.json"
	}
	b, err := ioutil.ReadFile(credFile)
	if err != nil {
		return fmt.Errorf("unable to read Gmail token file: %v", err)
	}

	// Parse credentials
	var creds map[string]interface{}
	if err := json.Unmarshal(b, &creds); err != nil {
		return fmt.Errorf("unable to parse Gmail token file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailSendScope, gmail.GmailReadonlyScope, gmail.GmailModifyScope)
	if err != nil {
		return fmt.Errorf("unable to parse client config: %v", err)
	}
	tok := &oauth2.Token{}
	if err := json.Unmarshal(b, tok); err != nil {
		return fmt.Errorf("unable to parse token: %v", err)
	}

	client := config.Client(ctx, tok)
	service, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to create Gmail service: %v", err)
	}
	GmailService = service
	log.Println("[OK] Gmail service initialized")
	return nil
} 