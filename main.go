package main

import (
	"fbm-vintage-monitor/config"
	"fbm-vintage-monitor/cookie"
	"fbm-vintage-monitor/db"
	"fbm-vintage-monitor/notifier"
	"fbm-vintage-monitor/parser"
	"fbm-vintage-monitor/scraper"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"
)

const (
	SleepStartHour = 0 // 00:00 WIB
	SleepEndHour   = 6 // 06:00 WIB
)

func init() {
	// Set timezone WIB (UTC+7) globally for logging if needed,
	// but standard time.Local is fine if OS is configured.
	// For simplicity, we just use local time and assume it's set correctly.
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err == nil {
		time.Local = loc
	}

	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	f, err := os.OpenFile("data/monitor.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		// Log to file and stdout (omitted multiple writers for brevity,
		// in production use io.MultiWriter if needed).
		// We'll stick to standard log for simplicity, or just write to stdout since service captures it.
		_ = f
	}
}

func isSleepHour() bool {
	now := time.Now()
	hour := now.Hour()
	return hour >= SleepStartHour && hour < SleepEndHour
}

func waitUntilAwake() {
	for isSleepHour() {
		now := time.Now()
		wakeTime := time.Date(now.Year(), now.Month(), now.Day(), SleepEndHour, 0, 0, 0, time.Local)
		if wakeTime.Before(now) {
			wakeTime = wakeTime.AddDate(0, 0, 1)
		}
		sleepDuration := wakeTime.Sub(now)
		log.Printf("😴 Jam tidur (00:00-06:00 WIB). Bangun dalam %.1f jam...", sleepDuration.Hours())
		time.Sleep(sleepDuration + time.Minute)
	}
}

func sendBlockAlert(status scraper.ResponseStatus, accountLabel, tgToken, tgChat string) {
	available := cookie.GlobalPool.GetAvailableCount()
	var msg string
	switch status {
	case scraper.StatusBlocked:
		msg = fmt.Sprintf("🚫 *AKUN [%s] DIBLOKIR*\n\nAkun ini akan di-skip otomatis.\nBuka Facebook → selesaikan verifikasi → update cookies/%s.json\n\nSisa akun aktif: %d", accountLabel, accountLabel, available)
	case scraper.StatusCheckpoint:
		msg = fmt.Sprintf("🔒 *CHECKPOINT [%s]*\n\nFacebook minta verifikasi.\nSelesaikan di browser → update cookies/%s.json\n\nSisa akun aktif: %d", accountLabel, accountLabel, available)
	case scraper.StatusLoginRequired:
		msg = fmt.Sprintf("🔑 *COOKIES EXPIRED [%s]*\n\nLogin ulang di browser → export cookies baru → cookies/%s.json\n\nSisa akun aktif: %d", accountLabel, accountLabel, available)
	default:
		msg = fmt.Sprintf("⚠️ *MASALAH [%s]*: %s", accountLabel, status)
	}
	notifier.SendAlert(msg, tgToken, tgChat)
}

func scanKeywords(conf *config.Config, keywords []string, groupName string) string {
	cookie.GlobalPool.ResetAllCycles()
	log.Printf("=== %s SCAN START ===", groupName)
	log.Printf("  Keywords: %d, Cities: %d, Akun tersedia: %d/%d", len(keywords), len(conf.Cities), cookie.GlobalPool.GetAvailableCount(), len(cookie.GlobalPool.Accounts))

	var allNew []db.Listing
	consecutiveUnavailable := 0
	requestsDone := 0

	for cityName, locationID := range conf.Cities {
		for _, keyword := range keywords {
			if isSleepHour() {
				waitUntilAwake()
			}

			account := cookie.GlobalPool.Next()
			if account == nil {
				consecutiveUnavailable++
				if consecutiveUnavailable >= 3 {
					msg := fmt.Sprintf("⚠️ *SEMUA AKUN HABIS/BLACKLISTED*\n\nTidak ada akun yang tersedia untuk scan %s.\nUpdate cookies dan restart bot.", groupName)
					log.Println(msg)
					notifier.SendAlert(msg, conf.TelegramToken, conf.TelegramChatID)
					return "ALL_EXHAUSTED"
				}
				log.Println("Semua akun sedang limit/blacklist, tunggu 5 menit...")
				time.Sleep(5 * time.Minute)
				cookie.GlobalPool.ResetAllCycles()
				continue
			}
			consecutiveUnavailable = 0

			log.Printf("[%s] [%s] '%s' @ %s", groupName, account.Label, keyword, cityName)

			// ── Human-like Behavioral Pause ──
			// Every 15-20 requests, take a long "coffee break" (3-5 minutes)
			if requestsDone > 0 && requestsDone%15 == 0 {
				breakTime := 180 + rand.Intn(120)
				log.Printf("☕ Coffee break... Istirahat sejenak selama %d detik agar terlihat natural.", breakTime)
				time.Sleep(time.Duration(breakTime) * time.Second)
			}

			doPreflight := account.RequestsThisCycle%8 == 0 // More frequent preflight
			result := scraper.FetchMarketplace(locationID, keyword, account.Cookies, 3, doPreflight, conf.ProxyURL)
			account.Use()
			requestsDone++

			if result.Status == scraper.StatusBlocked || result.Status == scraper.StatusCheckpoint || result.Status == scraper.StatusLoginRequired {
				account.Blacklist(string(result.Status))
				sendBlockAlert(result.Status, account.Label, conf.TelegramToken, conf.TelegramChatID)

				if cookie.GlobalPool.GetAvailableCount() > 0 {
					log.Printf("Akun [%s] di-blacklist, sisa %d akun. Lanjut...", account.Label, cookie.GlobalPool.GetAvailableCount())
					time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
					continue
				} else {
					log.Println("Semua akun ter-blacklist!")
					return "ALL_EXHAUSTED"
				}
			}

			if result.Status == scraper.StatusError || result.Status == scraper.StatusEmpty {
				time.Sleep(time.Duration(5+rand.Intn(11)) * time.Second)
				continue
			}

			listings := parser.ParseListings(result.HTML, cityName, keyword, conf.PriceMax)
			if len(listings) == 0 {
				time.Sleep(time.Duration(15+rand.Intn(16)) * time.Second)
				continue
			}

			newListings, _ := db.FilterNew(listings)
			if len(newListings) == 0 {
				time.Sleep(time.Duration(15+rand.Intn(16)) * time.Second)
				continue
			}

			_ = db.SaveListings(newListings)
			allNew = append(allNew, newListings...)

			delay := 15 + rand.Intn(16)
			log.Printf("Delay %ds...", delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}

		delay := 60 + rand.Intn(61)
		log.Printf("[%s] Selesai kota %s (%d requests), delay %ds...", groupName, cityName, requestsDone, delay)
		time.Sleep(time.Duration(delay) * time.Second)
	}

	if len(allNew) > 0 {
		log.Printf("📬 Sending %d notifications for %s...", len(allNew), groupName)
		notifier.SendBatch(allNew, conf.TelegramToken, conf.TelegramChatID)
	} else {
		log.Printf("%s scan selesai — tidak ada listing baru", groupName)
	}

	log.Println(cookie.GlobalPool.GetStatusReport())

	return fmt.Sprintf("%d", len(allNew))
}

func hotLoop(conf *config.Config) {
	interval := time.Duration(conf.PollIntervalHotHours*3600) * time.Second
	for {
		waitUntilAwake()
		result := scanKeywords(conf, conf.KeywordsHot, "HOT")

		if result == "ALL_EXHAUSTED" {
			log.Println("HOT: Semua akun exhausted, pause 6 jam...")
			time.Sleep(6 * time.Hour)
			cookie.GlobalPool.Reload()
			continue
		}

		log.Printf("HOT Loop sleeping %.1fh...", conf.PollIntervalHotHours)
		time.Sleep(interval)
	}
}

func generalLoop(conf *config.Config) {
	interval := time.Duration(conf.PollIntervalGeneralHours*3600) * time.Second
	for {
		waitUntilAwake()
		result := scanKeywords(conf, conf.KeywordsGeneral, "GENERAL")

		if result == "ALL_EXHAUSTED" {
			log.Println("GENERAL: Semua akun exhausted, pause 6 jam...")
			time.Sleep(6 * time.Hour)
			cookie.GlobalPool.Reload()
			continue
		}

		log.Printf("GENERAL Loop sleeping %.1fh...", conf.PollIntervalGeneralHours)
		time.Sleep(interval)
	}
}

func heartbeatLoop(conf *config.Config) {
	msg := fmt.Sprintf("🤖 *FBM Monitor v2 (Go Edition)*\n\nAkun aktif: %d/%d\nJam tidur: %02d:00-%02d:00 WIB\nSiap memindai listing vintage!", cookie.GlobalPool.GetAvailableCount(), len(cookie.GlobalPool.Accounts), SleepStartHour, SleepEndHour)
	notifier.SendAlert(msg, conf.TelegramToken, conf.TelegramChatID)

	for {
		time.Sleep(24 * time.Hour)
		hbMsg := fmt.Sprintf("🤖 *Heartbeat*\n\n%s", cookie.GlobalPool.GetStatusReport())
		notifier.SendAlert(hbMsg, conf.TelegramToken, conf.TelegramChatID)
	}
}

func startupHealthCheck(conf *config.Config) int {
	log.Println("🔍 Startup health check — mengecek semua akun...")
	okCount := 0

	for _, account := range cookie.GlobalPool.Accounts {
		result := scraper.CheckCookiesHealth(account.Cookies, conf.ProxyURL)
		if result.Status == scraper.StatusOK {
			log.Printf("  ✅ [%s] OK", account.Label)
			okCount++
		} else {
			log.Printf("  ❌ [%s] %s: %s", account.Label, result.Status, result.Detail)
			account.Blacklist(string(result.Status))
		}
		time.Sleep(time.Duration(5+rand.Intn(11)) * time.Second)
	}

	msg := fmt.Sprintf("🔍 *Health Check Selesai*\n\n✅ OK: %d/%d akun\n❌ Bermasalah: %d\n\n", okCount, len(cookie.GlobalPool.Accounts), len(cookie.GlobalPool.Accounts)-okCount)
	if okCount == 0 {
		msg += "⚠️ *TIDAK ADA AKUN YANG VALID!* Update cookies dulu sebelum bot bisa scan."
	} else if okCount < len(cookie.GlobalPool.Accounts) {
		msg += "Bot akan jalan dengan akun yang tersedia."
	} else {
		msg += "Semua akun siap! 🚀"
	}

	notifier.SendAlert(msg, conf.TelegramToken, conf.TelegramChatID)
	log.Printf("Health check: %d/%d akun OK", okCount, len(cookie.GlobalPool.Accounts))
	return okCount
}

func main() {
	log.Println("=======================================================")
	log.Println("FBM Vintage Monitor v2 — Anti-Ban Edition (Go Rewrite)")
	log.Println("=======================================================")

	conf, err := config.LoadConfig("config.toml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cookie.GlobalPool.LoadAll()
	if len(cookie.GlobalPool.Accounts) == 0 {
		log.Fatal("Tidak ada akun! Taruh file cookies di folder cookies/ atau cookies.json")
	}

	os.MkdirAll("data", 0755)
	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	okCount := startupHealthCheck(conf)
	if okCount == 0 {
		log.Println("Semua akun bermasalah. Perbaiki cookies dulu.")
		log.Println("Bot tetap jalan dan akan retry tiap 6 jam...")
	}

	var wg sync.WaitGroup

	wg.Add(3)
	go func() { defer wg.Done(); hotLoop(conf) }()
	go func() { defer wg.Done(); generalLoop(conf) }()
	go func() { defer wg.Done(); heartbeatLoop(conf) }()

	wg.Wait()
}
