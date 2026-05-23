package notifier

import (
	"bytes"
	"encoding/json"
	"fbm-vintage-monitor/db"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func formatPrice(price int) string {
	// Simple formatting for Rp x.xxx.xxx
	s := fmt.Sprintf("%d", price)
	var res []string
	for i := len(s) - 1; i >= 0; i-- {
		if (len(s)-1-i)%3 == 0 && i != len(s)-1 {
			res = append([]string{"."}, res...)
		}
		res = append([]string{string(s[i])}, res...)
	}
	return "Rp " + strings.Join(res, "")
}

func grailedEstimate(keyword string) string {
	estimates := map[string]string{
		"levis 501":                 "$35–65",
		"levis vintage":             "$30–60",
		"lvc 501":                   "$80–150",
		"single stitch":             "$25–80",
		"band tee":                  "$40–150",
		"harley davidson":           "$45–120",
		"nike vintage":              "$30–90",
		"nike acg":                  "$50–150",
		"y2k":                       "$20–60",
		"varsity jacket":            "$60–200",
		"carhartt":                  "$40–120",
		"polo ralph lauren vintage": "$30–90",
		"rlx polo":                  "$40–100",
		"nautica vintage":           "$25–70",
		"patagonia":                 "$50–180",
		"columbia vintage":          "$30–90",
	}
	if est, ok := estimates[strings.ToLower(keyword)]; ok {
		return est
	}
	return "$25–100"
}

func getUpperEstimate(est string) string {
	for _, delim := range []string{"–", "-"} {
		if strings.Contains(est, delim) {
			parts := strings.Split(est, delim)
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return est
}

func buildMessage(listing db.Listing) string {
	est := grailedEstimate(listing.Keyword)
	priceFmt := formatPrice(listing.PriceIDR)
	priceUSDApprox := float64(listing.PriceIDR) / 16000.0
	upperEst := getUpperEstimate(est)

	return fmt.Sprintf("🔔 *LISTING BARU — %s*\n\n📦 *%s*\n💰 %s (~$%.0f)\n📍 %s\n\n🔗 [Lihat di Marketplace](%s)\n\n💡 *Est. Grailed:* %s\n📈 Est. margin: $%.0f → %s",
		strings.ToUpper(listing.Keyword),
		listing.Title,
		priceFmt,
		priceUSDApprox,
		listing.City,
		listing.URL,
		est,
		priceUSDApprox,
		upperEst,
	)
}

func SendNotification(listing db.Listing, token, chatID string) {
	msg := buildMessage(listing)
	payload := map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     msg,
		"parse_mode":               "Markdown",
		"disable_web_page_preview": false,
	}
	sendTelegramPost(token, payload)
}

func SendAlert(message, token, chatID string) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}
	sendTelegramPost(token, payload)
}

func SendBatch(listings []db.Listing, token, chatID string) {
	for _, l := range listings {
		SendNotification(l, token, chatID)
		time.Sleep(500 * time.Millisecond)
	}
}

func sendTelegramPost(token string, payload map[string]interface{}) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Telegram error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("Telegram alert error %d", resp.StatusCode)
	}
}
