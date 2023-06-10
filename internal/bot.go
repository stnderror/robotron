package internal

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
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

func NewBot() (*Bot, error) {
	bot, err := telegram.NewBotAPI(mustGetEnv("ROBOTRON_TELEGRAM_TOKEN"))
	if err != nil {
		return nil, err
	}

	allowedUsers := make(map[int64]bool)

	ids := strings.Split(mustGetEnv("ROBOTRON_ALLOWED_USERS"), ",")
	for _, id := range ids {
		id, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, err
		}
		allowedUsers[id] = true
	}

	if len(allowedUsers) == 0 {
		return nil, errors.New("no allowed users")
	}

	return &Bot{bot, NewAI(), NewStore(), allowedUsers}, nil
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
	switch msg.Command() {
	case "clear":
		b.store.Clear(msg.Chat.ID)
		log.WithField("from", msg.From).Info("Cleared thread.")
	default:
		log.WithFields(log.Fields{
			"from":    msg.From,
			"command": msg.Command(),
		}).Info("Skiping unsupported command.")
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

	cancel, err := b.notifyTyping(ctx, msg.Chat.ID)
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

	mp3, err := ioutil.TempFile(os.TempDir(), "voice-*.mp3")
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

func (b *Bot) notifyTyping(ctx context.Context, chatID int64) (func(), error) {
	if _, err := b.telegram.Request(telegram.NewChatAction(chatID, telegram.ChatTyping)); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(3 * time.Second)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, err := b.telegram.Request(telegram.NewChatAction(chatID, telegram.ChatTyping))
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

	file, err := ioutil.TempFile(os.TempDir(), "voice-*.ogg")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(file, res.Body)
	if err != nil {
		return nil, err
	}

	return file, nil
}
