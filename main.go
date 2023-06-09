package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/stnderror/robotron/internal"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)

	level, err := log.ParseLevel(os.Getenv("ROBOTRON_LOG_LEVEL"))
	if err != nil {
		log.SetLevel(log.InfoLevel)
	}
	log.SetLevel(level)
}

func main() {
	log.Info("Starting.")

	bot, err := internal.NewBot()
	if err != nil {
		log.Fatal(err)
	}

	if err := bot.Run(); err != nil {
		log.Fatal(err)
	}
}
