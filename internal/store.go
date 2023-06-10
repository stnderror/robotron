package internal

import (
	"time"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	defaultTTL = 3 * time.Hour
)

type Store struct {
	threads map[int64][]*telegram.Message
}

func NewStore() *Store {
	return &Store{
		threads: make(map[int64][]*telegram.Message),
	}
}

func (s *Store) Thread(chatID int64) []*telegram.Message {
	found, ok := s.threads[chatID]
	if !ok {
		return nil
	}

	res := []*telegram.Message{}
	for _, msg := range found {
		if time.Since(msg.Time()) < defaultTTL {
			res = append(res, msg)
		}
	}

	s.threads[chatID] = res
	return res
}

func (s *Store) Clear(chatID int64) {
	delete(s.threads, chatID)
}

func (s *Store) Put(msg *telegram.Message) {
	chatID := msg.Chat.ID
	if _, ok := s.threads[chatID]; !ok {
		s.threads[chatID] = []*telegram.Message{}
	}

	s.threads[chatID] = append(s.threads[chatID], msg)
}
