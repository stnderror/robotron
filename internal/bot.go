package internal

import (
	"context"
	"errors"
	"io"
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
	telegram *telegram.BotAPI
	ai       *AI
}

func NewBot() (*Bot, error) {
	bot, err := telegram.NewBotAPI(MustGetEnv("ROBOTRON_TELEGRAM_TOKEN"))
	if err != nil {
		return nil, err
	}
	return &Bot{bot, NewAI()}, nil
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

	switch {
	case msg.IsCommand():
		return b.handleCommand(ctx, msg)
	case msg.Text != "":
		return b.handleText(ctx, msg)
	case msg.Audio != nil:
		return b.handleAudio(ctx, msg)
	default:
		log.Info("Skiping unsupported message.")
		return nil
	}
}

func (b *Bot) handleCommand(ctx context.Context, msg *telegram.Message) error {
	return nil
}

func (b *Bot) handleText(ctx context.Context, msg *telegram.Message) error {
	stream, err := b.ai.StreamingReply(ctx, msg.Text)
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

	_, err = b.streamMessage(msg.Chat.ID, reply, strings.Join(delta, ""))
	return err
}

func (b *Bot) handleAudio(ctx context.Context, msg *telegram.Message) error {
	return nil
}

func (b *Bot) streamMessage(chatID int64, msg *telegram.Message, delta string) (*telegram.Message, error) {
	if strings.TrimSpace(delta) == "" {
		return nil, nil
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
