package internal

import (
	"github.com/caarlos0/env/v8"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	TelegramToken string    `env:"TELEGRAM_TOKEN,notEmpty"`
	OpenAIAPIKey  string    `env:"OPENAI_API_KEY,notEmpty"`
	AllowedUsers  []int64   `env:"ALLOWED_USERS,notEmpty"`
	MeasureUnits  string    `env:"MEASURE_UNITS" envDefault:"metric"`
	LogLevel      log.Level `env:"LOG_LEVEL" envDefault:"INFO"`
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{}
	return cfg, env.ParseWithOptions(&cfg, env.Options{
		Prefix: "ROBOTRON_",
	})
}
