// Package main - notifications.go
// Multi-channel notification system for Remote Bridge
// Supports: Telegram, WhatsApp (CallMeBot)

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// NotificationConfig holds all notification channel configurations
type NotificationConfig struct {
	// Telegram
	TelegramBotToken string
	TelegramChatID   string

	// WhatsApp (CallMeBot - Free API)
	WhatsAppAPIKey string
	UserPhone      string
}

// notifyConfig is the global notification configuration
var notifyConfig NotificationConfig

// initNotifications loads notification configuration from environment
func initNotifications() {
	notifyConfig = NotificationConfig{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		WhatsAppAPIKey:   os.Getenv("WHATSAPP_API_KEY"),
		UserPhone:        os.Getenv("USER_PHONE"),
	}

	// Log which channels are configured
	channels := []string{}
	if notifyConfig.TelegramBotToken != "" && notifyConfig.TelegramChatID != "" {
		channels = append(channels, "Telegram")
	}
	if notifyConfig.WhatsAppAPIKey != "" && notifyConfig.UserPhone != "" {
		channels = append(channels, "WhatsApp (CallMeBot)")
		fmt.Fprintf(os.Stderr, "[BRIDGE] 🟢 WhatsApp configured for: %s\n", notifyConfig.UserPhone)
	}

	if len(channels) > 0 {
		fmt.Fprintf(os.Stderr, "[BRIDGE] 📢 Notification channels enabled: %s\n", strings.Join(channels, ", "))
	} else {
		fmt.Fprintln(os.Stderr, "[BRIDGE] ⚠️  No notification channels configured")
	}
}

// broadcastNotification sends notifications to all configured channels
func broadcastNotification(question string, options []string, remoteURL string, requestID string) {
	fmt.Fprintf(os.Stderr, "[BRIDGE] 📤 Broadcasting notification to all channels...\n")
	var wg sync.WaitGroup

	// Channel A: Telegram
	if notifyConfig.TelegramBotToken != "" && notifyConfig.TelegramChatID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sendTelegramNotification(question, options, remoteURL); err != nil {
				fmt.Fprintf(os.Stderr, "[BRIDGE] ⚠️  Telegram failed: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "[BRIDGE] 📱 Telegram notification sent!")
			}
		}()
	}

	// Channel B: WhatsApp (CallMeBot)
	if notifyConfig.WhatsAppAPIKey != "" && notifyConfig.UserPhone != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Fprintln(os.Stderr, "[BRIDGE] 📲 WhatsApp: Starting send...")
			if err := sendWhatsAppNotification(question, options, remoteURL, requestID); err != nil {
				fmt.Fprintf(os.Stderr, "[BRIDGE] ⚠️  WhatsApp failed: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "[BRIDGE] 📲 WhatsApp notification sent!")
			}
		}()
	}

	// Wait for all notifications to complete
	wg.Wait()
	fmt.Fprintln(os.Stderr, "[BRIDGE] ✅ All notification channels completed")
}

// sendTelegramNotification sends a message via Telegram Bot API with single link
func sendTelegramNotification(question string, options []string, remoteURL string) error {
	if telegramBot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	// Don't escape question - it breaks markdown link syntax
	// Single "Tap to Decide" link (no answer in URL - opens interactive form)
	message := fmt.Sprintf("🤖 *Agent Paused*\n\n❓ %s\n\n👇 *Choose an option:*\n\n👉 [Tap to Decide](%s)", question, remoteURL)

	msg := tgbotapi.NewMessage(telegramChatID, message)
	msg.ParseMode = "Markdown"

	_, err := telegramBot.Send(msg)
	return err
}

// sendWhatsAppNotification sends a message via CallMeBot API
func sendWhatsAppNotification(question string, options []string, remoteURL string, requestID string) error {
	// Construct the message
// CallMeBot supports basic formatting: *bold*, _italic_, %0A for new line
	
	// Format:
	// 🤖 Agent Paused
	// ❓ Question
	//
	// 👉 Tap to Decide: <URL>
	
	// We need to encode specifically for URL parameters
	// But first let's build the text string
	
	messageLines := []string{
		"🤖 *Agent Paused*",
		fmt.Sprintf("❓ %s", question),
		"",
		fmt.Sprintf("👉 Tap to Decide: %s", remoteURL),
	}
	
	fullMessage := strings.Join(messageLines, "\n")
	
	// Create the request URL
	baseURL := "https://api.callmebot.com/whatsapp.php"
	params := url.Values{}
	params.Add("phone", notifyConfig.UserPhone)
	params.Add("text", fullMessage)
	params.Add("apikey", notifyConfig.WhatsAppAPIKey)
	
	finalURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// Send GET request
	resp, err := http.Get(finalURL)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("CallMeBot API returned status %d", resp.StatusCode)
	}

	return nil
}

// escapeMarkdown escapes special characters for Telegram Markdown
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"`", "\\`",
	)
	return replacer.Replace(s)
}
