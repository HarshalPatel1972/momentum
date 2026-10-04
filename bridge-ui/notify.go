package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var notifyClient = &http.Client{Timeout: 20 * time.Second}

// notify sends a link-based question (WhatsApp). Telegram questions use
// answer buttons instead; see telegram.go. An error means the user was not
// reached, so the caller should fail fast instead of waiting.
func notify(cfg BridgeConfig, q *pendingQuestion, link string) error {
	switch cfg.Channel {
	case "whatsapp":
		return sendWhatsApp(cfg.WhatsApp, q, link)
	default:
		return fmt.Errorf("%s", configProblem(cfg))
	}
}

func telegramAPIBase() string {
	if b := os.Getenv("MOMENTUM_TELEGRAM_API"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return "https://api.telegram.org"
}

func sourceLabel(q *pendingQuestion) string {
	parts := []string{}
	if q.Client != "" {
		parts = append(parts, q.Client)
	}
	if q.Project != "" {
		parts = append(parts, q.Project)
	}
	return strings.Join(parts, " · ")
}

func sendWhatsApp(wa WhatsAppConfig, q *pendingQuestion, link string) error {
	text := "🤖 *Input Needed*"
	if src := sourceLabel(q); src != "" {
		text += "\n_" + src + "_"
	}
	text += "\n\n" + q.Question + "\n\n👉 Tap to respond: " + link
	params := url.Values{}
	params.Set("phone", wa.Phone)
	params.Set("text", text)
	params.Set("apikey", wa.APIKey)
	resp, err := notifyClient.Get("https://api.callmebot.com/whatsapp.php?" + params.Encode())
	if err != nil {
		return fmt.Errorf("whatsapp: %v", redactToken(err.Error(), wa.APIKey))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("whatsapp: CallMeBot returned %s", resp.Status)
	}
	return nil
}

func redactToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "***")
}
