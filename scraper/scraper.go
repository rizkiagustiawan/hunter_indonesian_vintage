package scraper

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

type ResponseStatus string

const (
	StatusOK            ResponseStatus = "ok"
	StatusBlocked       ResponseStatus = "blocked"
	StatusCheckpoint    ResponseStatus = "checkpoint"
	StatusLoginRequired ResponseStatus = "login"
	StatusEmpty         ResponseStatus = "empty"
	StatusError         ResponseStatus = "error"
)

type FetchResult struct {
	Status ResponseStatus
	HTML   string
	Detail string
}

var (
	blockSignals = []string{
		"anda diblokir sementara",
		"you're temporarily blocked",
		"temporarily blocked",
		"your account has been locked",
		"akun anda telah dikunci",
	}
	checkpointSignals = []string{
		"/checkpoint/",
		"verify your identity",
		"verifikasi identitas",
		"confirm your identity",
		"konfirmasi identitas anda",
	}
	loginSignals = []string{
		"login_form",
		"loginfooter",
		`<input name="email"`,
	}
	marketplaceSignals = []string{
		"marketplace_listing_title",
		"marketplace_search",
		"MarketplaceListing",
		"marketplace_listing_renderable_target",
	}
)

func diagnoseResponse(html string) ResponseStatus {
	htmlLower := strings.ToLower(html)

	for _, sig := range blockSignals {
		if strings.Contains(htmlLower, sig) {
			return StatusBlocked
		}
	}

	for _, sig := range checkpointSignals {
		if strings.Contains(htmlLower, sig) {
			return StatusCheckpoint
		}
	}

	// Limit login check to first 50k chars to match python logic
	checkLen := 50000
	if len(htmlLower) < checkLen {
		checkLen = len(htmlLower)
	}
	for _, sig := range loginSignals {
		if strings.Contains(htmlLower[:checkLen], sig) {
			return StatusLoginRequired
		}
	}

	for _, sig := range marketplaceSignals {
		// Python script checked original HTML case here, but strings.Contains is fine
		if strings.Contains(html, sig) {
			return StatusOK
		}
	}

	return StatusEmpty
}

func preflightHomepage(client *req.Client) {
	_, _ = client.R().Get("https://www.facebook.com/")
	time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)
}

func FetchMarketplace(locationID, keyword string, cookies map[string]string, retries int, doPreflight bool, proxyURL string) FetchResult {
	url := fmt.Sprintf("https://www.facebook.com/marketplace/%s/search/?query=%s&sortBy=creation_time_descend", locationID, keyword)

	for attempt := 0; attempt < retries; attempt++ {
		// Create a new client per attempt for fresh fingerprinting
		client := req.C().
			ImpersonateChrome().
			EnableHTTP3(). // 2026 standard: use HTTP/3 whenever possible
			SetTimeout(30 * time.Second).
			SetCommonHeaders(map[string]string{
				"Accept-Language":           "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7",
				"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
				"Sec-Ch-Ua":                 `"Not(A:Brand";v="99", "Google Chrome";v="120", "Chromium";v="120"`,
				"Sec-Ch-Ua-Mobile":          "?0",
				"Sec-Ch-Ua-Platform":        `"Windows"`,
				"Sec-Fetch-Dest":            "document",
				"Sec-Fetch-Mode":            "navigate",
				"Sec-Fetch-Site":            "none",
				"Sec-Fetch-User":            "?1",
				"Upgrade-Insecure-Requests": "1",
				"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			})

		if proxyURL != "" {
			client.SetProxyURL(proxyURL)
		}

		var httpCookies []*http.Cookie
		for k, v := range cookies {
			httpCookies = append(httpCookies, &http.Cookie{
				Name:   k,
				Value:  v,
				Domain: ".facebook.com",
			})
		}
		client.SetCommonCookies(httpCookies...)

		if doPreflight {
			preflightHomepage(client)
		}

		resp, err := client.R().Get(url)
		if err != nil {
			log.Printf("Attempt %d/%d failed for '%s': %v\n", attempt+1, retries, keyword, err)
			time.Sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}

		if resp.StatusCode != 200 {
			log.Printf("Non-200: %d for '%s'@%s\n", resp.StatusCode, keyword, locationID)
			return FetchResult{Status: StatusError, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
		}

		html := resp.String()
		if len(html) < 100000 {
			log.Printf("Response too small (%d bytes)\n", len(html))
			return FetchResult{Status: StatusLoginRequired, Detail: fmt.Sprintf("Only %d bytes", len(html))}
		}

		status := diagnoseResponse(html)

		if status == StatusOK {
			log.Printf("OK: %s@%s — %d bytes\n", keyword, locationID, len(html))
			return FetchResult{Status: StatusOK, HTML: html}
		}

		if status == StatusBlocked {
			log.Printf("🚫 BLOCKED: '%s'@%s\n", keyword, locationID)
			return FetchResult{Status: StatusBlocked, Detail: "Anda Diblokir Sementara"}
		}

		if status == StatusCheckpoint {
			log.Printf("🔒 CHECKPOINT: '%s'@%s\n", keyword, locationID)
			return FetchResult{Status: StatusCheckpoint, Detail: "Verifikasi diperlukan"}
		}

		if status == StatusLoginRequired {
			log.Printf("🔑 LOGIN EXPIRED: '%s'@%s\n", keyword, locationID)
			return FetchResult{Status: StatusLoginRequired, Detail: "Cookies expired"}
		}

		log.Printf("⚠️ EMPTY: %d bytes tapi tanpa data marketplace\n", len(html))
		return FetchResult{Status: StatusEmpty, HTML: html, Detail: "No marketplace data"}
	}

	log.Printf("All %d attempts failed for '%s'@%s\n", retries, keyword, locationID)
	return FetchResult{Status: StatusError, Detail: fmt.Sprintf("All %d attempts failed", retries)}
}

func CheckCookiesHealth(cookies map[string]string, proxyURL string) FetchResult {
	log.Println("🔍 Checking cookies health...")
	res := FetchMarketplace("106476472720278", "jaket", cookies, 2, true, proxyURL)

	statusMsg := map[ResponseStatus]string{
		StatusOK:            "✅ Cookies valid",
		StatusBlocked:       "🚫 AKUN DIBLOKIR",
		StatusCheckpoint:    "🔒 CHECKPOINT",
		StatusLoginRequired: "🔑 COOKIES EXPIRED",
		StatusEmpty:         "⚠️ Response kosong",
		StatusError:         "❌ Network error",
	}
	msg := statusMsg[res.Status]
	if msg == "" {
		msg = "?"
	}
	log.Printf("Health: %s\n", msg)
	return res
}
