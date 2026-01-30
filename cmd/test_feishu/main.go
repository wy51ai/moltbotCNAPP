package main

import (
	"log"
	"time"

	"github.com/wy51ai/moltbotCNAPP/internal/feishu"
)

func main() {
	appID := "cli_PLACEHOLDER" // TODO: Use env var or config
	appSecret := "SECRET_PLACEHOLDER" // TODO: Use env var or config
	chatID := "oc_PLACEHOLDER"

	client := feishu.NewClient(appID, appSecret, nil)

	log.Println("Sending test message...")
	msgID, err := client.SendMessage(chatID, "Debug Test: Starting...")
	if err != nil {
		log.Fatalf("SendMessage failed: %v", err)
	}
	log.Printf("Message sent: %s", msgID)

	for i := 1; i <= 3; i++ {
		time.Sleep(1 * time.Second)
		log.Printf("Updating message %d...", i)
		updateText := "Debug Test: Update " + string(rune('0'+i)) + " ▌"
		err = client.UpdateMessage(msgID, updateText)
		if err != nil {
			log.Printf("UpdateMessage failed: %v", err)
		} else {
			log.Println("Update success")
		}
	}

	time.Sleep(1 * time.Second)
	client.UpdateMessage(msgID, "Debug Test: Finished")
}
