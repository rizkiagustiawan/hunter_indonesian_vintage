package main

import (
	"fbm-vintage-monitor/config"
	"fbm-vintage-monitor/cookie"
	"fbm-vintage-monitor/scraper"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	fmt.Println("======================================")
	fmt.Println("   FBM COOKIE HEALTH CHECKER (GO)     ")
	fmt.Println("======================================")

	conf, err := config.LoadConfig("config.toml")
	if err != nil {
		log.Printf("Warning: config.toml not loaded, proxy will be disabled: %v\n", err)
		conf = &config.Config{}
	}

	cookie.GlobalPool.LoadAll()
	if len(cookie.GlobalPool.Accounts) == 0 {
		fmt.Println("❌ Error: Tidak ada cookies ditemukan di folder 'cookies/'")
		os.Exit(1)
	}

	fmt.Printf("Ditemukan %d akun. Memulai pengecekan...\n\n", len(cookie.GlobalPool.Accounts))

	live := 0
	die := 0

	for _, acc := range cookie.GlobalPool.Accounts {
		fmt.Printf("Checking [%s] ... ", acc.Label)

		result := scraper.CheckCookiesHealth(acc.Cookies, conf.ProxyURL)

		if result.Status == scraper.StatusOK {
			fmt.Printf("✅ LIVE\n")
			live++
		} else {
			fmt.Printf("❌ DIE (%s: %s)\n", result.Status, result.Detail)
			die++
		}

		// Jeda sedikit biar tidak dianggap spam saat check banyak cookies
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n======================================")
	fmt.Printf("HASIL AKHIR:\n")
	fmt.Printf("🟢 LIVE: %d\n", live)
	fmt.Printf("🔴 DIE : %d\n", die)
	fmt.Println("======================================")
}
