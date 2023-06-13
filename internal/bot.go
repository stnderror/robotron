package internal

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

const (
	handlingTimeout    = 60 * time.Second
	streamingChunkSize = 10
)

type Bot struct {
	telegram     *telegram.BotAPI
	ai           *AI
	store        *Store
	allowedUsers map[int64]bool
}

func NewBot(cfg Config) (*Bot, error) {
	tg, err := telegram.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, err
	}

	if err := registerCommands(tg, map[string]string{
		"clear":   "Clears current chat",
		"imagine": "Generates an image",
	}); err != nil {
		return nil, err
	}

	allowedUsers := make(map[int64]bool)
	for _, id := range cfg.AllowedUsers {
		allowedUsers[id] = true
	}

	return &Bot{tg, NewAI(cfg), NewStore(), allowedUsers}, nil
}

func (b *Bot) Run() error {
	update := telegram.NewUpdate(0)
	update.Timeout = 60
	for update := range b.telegram.GetUpdatesChan(update) {
		if update.Message == nil {
			log.Info("Skiping unsupported update.")
			continue
		}
		if err := b.handle(update.Message); err != nil {
			return err
		}
		log.WithField("from", update.Message.From).Info("Handled message.")
	}
	return nil
}

func (b *Bot) handle(msg *telegram.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), handlingTimeout)
	defer cancel()

	if !b.allowedUsers[msg.From.ID] {
		log.WithField("from", msg.From).Info("User is not allowed.")
		return nil
	}

	switch {
	case msg.IsCommand():
		return b.handleCommand(ctx, msg)
	case msg.Text != "":
		return b.handleText(ctx, msg)
	case msg.Voice != nil:
		return b.handleVoice(ctx, msg)
	default:
		log.Info("Skiping unsupported message.")
		return nil
	}
}

func (b *Bot) handleCommand(ctx context.Context, msg *telegram.Message) error {
	log.WithFields(
		log.Fields{
			"from":    msg.From,
			"command": msg.Command(),
			"args":    msg.CommandArguments(),
		},
	).Info("Handling command.")

	switch msg.Command() {
	case "clear":
		return b.handleClearCommand(ctx, msg)
	case "imagine":
		return b.handleImagineCommand(ctx, msg)
	default:
		log.WithFields(log.Fields{
			"from":    msg.From,
			"command": msg.Command(),
			"args":    msg.CommandArguments(),
		}).Info("Skipped unsupported command.")
	}
	return nil
}

func (b *Bot) handleClearCommand(ctx context.Context, msg *telegram.Message) error {
	b.store.Clear(msg.Chat.ID)
	return nil
}

func (b *Bot) handleImagineCommand(ctx context.Context, msg *telegram.Message) error {
	prompt := msg.CommandArguments()
	if prompt == "" {
		_, err := b.message(msg.Chat.ID, "Please provide a prompt.")
		return err
	}

	cancel, err := b.notify(ctx, msg.Chat.ID, telegram.ChatUploadPhoto)
	if err != nil {
		return err
	}
	defer cancel()

	imgs, err := b.ai.Imagine(ctx, prompt)
	if err != nil {
		return err
	}

	files := make([]any, len(imgs))
	for i, img := range imgs {
		files[i] = telegram.NewInputMediaPhoto(telegram.FileURL(img))
	}

	if _, err := b.telegram.Request(telegram.NewMediaGroup(msg.Chat.ID, files)); err != nil {
		return err
	}
	return nil
}

func (b *Bot) handleText(ctx context.Context, msg *telegram.Message) error {
	b.store.Put(msg)

	thread := b.store.Thread(msg.Chat.ID)
	log.WithFields(log.Fields{
		"from":        msg.From,
		"thread_size": len(thread),
	}).Debug("Handling text.")

	stream, err := b.ai.StreamingReply(ctx, thread)
	if err != nil {
		return err
	}

	cancel, err := b.notify(ctx, msg.Chat.ID, telegram.ChatTyping)
	if err != nil {
		return err
	}
	defer cancel()

	var (
		reply *telegram.Message
	)

	delta := []string{}
	for chunk := range stream {
		cancel()

		if chunk.Error != nil {
			if errors.Is(chunk.Error, io.EOF) {
				break
			}
			return chunk.Error
		}

		delta = append(delta, chunk.Delta)
		if len(delta) < streamingChunkSize {
			continue
		}

		reply, err = b.streamMessage(msg.Chat.ID, reply, strings.Join(delta, ""))
		if err != nil {
			return err
		}
		delta = []string{}
	}

	reply, err = b.streamMessage(msg.Chat.ID, reply, strings.Join(delta, ""))
	if err != nil {
		return err
	}

	b.store.Put(reply)
	return nil
}

func (b *Bot) handleVoice(ctx context.Context, msg *telegram.Message) error {
	ogg, err := b.downloadFile(msg.Voice.FileID)
	if err != nil {
		return err
	}
	defer os.Remove(ogg.Name())

	mp3, err := os.CreateTemp(os.TempDir(), "voice-*.mp3")
	if err != nil {
		return err
	}
	defer os.Remove(mp3.Name())

	if err := transcode(ogg, mp3); err != nil {
		return err
	}

	text, err := b.ai.Transcribe(ctx, mp3)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"from":            msg.From,
		"voice_size":      msg.Voice.FileSize,
		"voice_duration":  msg.Voice.Duration,
		"voice_mime_type": msg.Voice.MimeType,
		"text":            text,
	}).Debug("Transcribed voice.")

	msg.Text = text
	return b.handleText(ctx, msg)
}

func (b *Bot) message(chatID int64, text string) (telegram.Message, error) {
	return b.telegram.Send(telegram.NewMessage(chatID, text))
}

func (b *Bot) streamMessage(chatID int64, msg *telegram.Message, delta string) (*telegram.Message, error) {
	if strings.TrimSpace(delta) == "" {
		return msg, nil
	}

	if msg == nil {
		sent, err := b.telegram.Send(telegram.NewMessage(chatID, delta))
		return &sent, err
	}

	sent, err := b.telegram.Send(telegram.NewEditMessageText(chatID, msg.MessageID, msg.Text+delta))
	return &sent, err
}

func (b *Bot) notify(ctx context.Context, chatID int64, action string) (func(), error) {
	if _, err := b.telegram.Request(telegram.NewChatAction(chatID, action)); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(3 * time.Second)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, err := b.telegram.Request(telegram.NewChatAction(chatID, action))
				if err != nil {
					log.WithError(err).Warn("Failed to send chat action.")
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return cancel, nil
}

func (b *Bot) downloadFile(fileID string) (*os.File, error) {
	url, err := b.telegram.GetFileDirectURL(fileID)
	if err != nil {
		return nil, err
	}

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	file, err := os.CreateTemp(os.TempDir(), "voice-*.ogg")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(file, res.Body)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func registerCommands(tg *telegram.BotAPI, commands map[string]string) error {
	cmds := []telegram.BotCommand{}
	for name, description := range commands {
		cmds = append(cmds, telegram.BotCommand{
			Command:     name,
			Description: description,
		})
	}
	_, err := tg.Request(telegram.NewSetMyCommandsWithScope(
		telegram.NewBotCommandScopeAllPrivateChats(), cmds...,
	))
	return err
}
