package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/stnderror/robotron/internal"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

func main() {
	log.Info("Starting.")

	cfg, err := internal.ConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	log.SetLevel(cfg.LogLevel)

	bot, err := internal.NewBot(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if err := bot.Run(); err != nil {
		log.Fatal(err)
	}
}
