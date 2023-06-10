package internal

import (
	"fmt"
	"os"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func mustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Errorf("%s not found", key))
	}
	return value
}

func transcode(in, out *os.File) error {
	return ffmpeg.Input(in.Name(), ffmpeg.KwArgs{"loglevel": "panic"}).
		Output(out.Name()).
		OverWriteOutput().
		Silent(true).
		ErrorToStdOut().
		Run()
}
