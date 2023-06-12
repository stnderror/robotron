package internal

import (
	"bytes"
	"os"
	"text/template"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func transcode(in, out *os.File) error {
	return ffmpeg.Input(in.Name(), ffmpeg.KwArgs{"loglevel": "panic"}).
		Output(out.Name()).
		OverWriteOutput().
		Silent(true).
		ErrorToStdOut().
		Run()
}

func mustRender(tmpl *template.Template, data any) string {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}
