package config

import (
	"log"

	"github.com/BurntSushi/toml"
)

type Config struct {
	PollIntervalHotHours     float64           `toml:"poll_interval_hot_hours"`
	PollIntervalGeneralHours float64           `toml:"poll_interval_general_hours"`
	PriceMax                 int               `toml:"price_max"`
	ProxyURL                 string            `toml:"proxy_url"`
	TelegramToken            string            `toml:"telegram_token"`
	TelegramChatID           string            `toml:"telegram_chat_id"`
	KeywordsHot              []string          `toml:"keywords_hot"`
	KeywordsGeneral          []string          `toml:"keywords_general"`
	Cities                   map[string]string `toml:"cities"`
}

func LoadConfig(path string) (*Config, error) {
	var conf Config
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		log.Printf("Error loading config: %v\n", err)
		return nil, err
	}
	return &conf, nil
}
