package cookie

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxRequestsPerAccountPerCycle = 25

type Account struct {
	Filepath          string
	Cookies           map[string]string
	Label             string
	RequestsThisCycle int
	TotalRequests     int
	IsBlacklisted     bool
	BlacklistReason   string
	LastUsed          time.Time
}

func (a *Account) ResetCycle() {
	a.RequestsThisCycle = 0
}

func (a *Account) Use() {
	a.RequestsThisCycle++
	a.TotalRequests++
	a.LastUsed = time.Now().UTC()
}

func (a *Account) Blacklist(reason string) {
	a.IsBlacklisted = true
	a.BlacklistReason = reason
	log.Printf("🚫 Akun [%s] di-blacklist: %s\n", a.Label, reason)
}

func (a *Account) Unblacklist() {
	a.IsBlacklisted = false
	a.BlacklistReason = ""
	log.Printf("✅ Akun [%s] di-unblacklist\n", a.Label)
}

func (a *Account) IsAvailable() bool {
	if a.IsBlacklisted {
		return false
	}
	if a.RequestsThisCycle >= MaxRequestsPerAccountPerCycle {
		return false
	}
	return true
}

type Pool struct {
	Accounts []*Account
	index    int
}

var GlobalPool = &Pool{}

func (p *Pool) LoadAll() {
	cookieDir := "cookies"
	if _, err := os.Stat(cookieDir); os.IsNotExist(err) {
		log.Printf("Folder '%s' tidak ditemukan\n", cookieDir)
		return
	}

	files, err := os.ReadDir(cookieDir)
	if err != nil || len(files) == 0 {
		log.Printf("Tidak ada file .json di '%s/'\n", cookieDir)
		// Fallback
		fallback := "cookies.json"
		if _, err := os.Stat(fallback); err == nil {
			data, err := os.ReadFile(fallback)
			if err == nil {
				var cookies map[string]string
				if err := json.Unmarshal(data, &cookies); err == nil {
					p.Accounts = append(p.Accounts, &Account{
						Filepath: fallback,
						Cookies:  cookies,
						Label:    "default",
					})
					log.Println("Loaded fallback cookies.json")
				}
			}
		}
		return
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		fp := filepath.Join(cookieDir, f.Name())
		data, err := os.ReadFile(fp)
		if err != nil {
			log.Printf("Gagal load %s: %v\n", f.Name(), err)
			continue
		}
		var cookies map[string]string
		if err := json.Unmarshal(data, &cookies); err != nil {
			log.Printf("Skip %s: bukan format yang valid\n", f.Name())
			continue
		}
		if _, ok := cookies["c_user"]; !ok {
			log.Printf("Skip %s: c_user tidak ada\n", f.Name())
			continue
		}
		p.Accounts = append(p.Accounts, &Account{
			Filepath: fp,
			Cookies:  cookies,
			Label:    strings.TrimSuffix(f.Name(), ".json"),
		})
		cUser := cookies["c_user"]
		if len(cUser) > 6 {
			cUser = cUser[:6]
		}
		log.Printf("Loaded cookies [%s] (c_user: %s...)\n", strings.TrimSuffix(f.Name(), ".json"), cUser)
	}
	log.Printf("Cookie pool ready: %d akun loaded\n", len(p.Accounts))
}

func (p *Pool) Next() *Account {
	if len(p.Accounts) == 0 {
		return nil
	}
	for i := 0; i < len(p.Accounts); i++ {
		account := p.Accounts[p.index]
		p.index = (p.index + 1) % len(p.Accounts)
		if account.IsAvailable() {
			return account
		}
	}
	return nil
}

func (p *Pool) ResetAllCycles() {
	for _, a := range p.Accounts {
		a.ResetCycle()
	}
}

func (p *Pool) GetAvailableCount() int {
	count := 0
	for _, a := range p.Accounts {
		if a.IsAvailable() {
			count++
		}
	}
	return count
}

func (p *Pool) GetStatusReport() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Cookie Pool Status (%d akun):", len(p.Accounts)))
	for _, acc := range p.Accounts {
		status := "✅ OK"
		if acc.IsBlacklisted {
			status = fmt.Sprintf("🚫 BLACKLISTED (%s)", acc.BlacklistReason)
		} else if !acc.IsAvailable() {
			status = fmt.Sprintf("⏸ LIMIT (%d/%d)", acc.RequestsThisCycle, MaxRequestsPerAccountPerCycle)
		}
		cUser := acc.Cookies["c_user"]
		if len(cUser) > 6 {
			cUser = cUser[:6]
		}
		lines = append(lines, fmt.Sprintf("  [%s] c_user=%s… cycle=%d total=%d → %s",
			acc.Label, cUser, acc.RequestsThisCycle, acc.TotalRequests, status))
	}
	return strings.Join(lines, "\n")
}

func (p *Pool) Reload() {
	oldBlacklist := make(map[string]string)
	for _, a := range p.Accounts {
		if a.IsBlacklisted {
			oldBlacklist[a.Label] = a.BlacklistReason
		}
	}
	p.Accounts = nil
	p.index = 0
	p.LoadAll()
	for _, a := range p.Accounts {
		if reason, ok := oldBlacklist[a.Label]; ok {
			a.Blacklist(reason)
		}
	}
}
