package internal

import (
	"context"
	"fmt"
	"os"
	"time"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openai "github.com/sashabaranov/go-openai"
)

const SystemPrompt = `You are Robotron, a personal robot assistant.
Today is %s.
`

type AI struct {
	openai *openai.Client
}

func NewAI() *AI {
	return &AI{openai.NewClient(mustGetEnv("ROBOTRON_OPENAI_API_KEY"))}
}

type StreamChunk struct {
	Delta string
	Error error
}

func (a *AI) Transcribe(ctx context.Context, file *os.File) (string, error) {
	res, err := a.openai.CreateTranscription(ctx, openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: file.Name(),
		Format:   openai.AudioResponseFormatJSON,
	})
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

func (a *AI) StreamingReply(ctx context.Context, thread []*telegram.Message) (chan StreamChunk, error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: fmt.Sprintf(SystemPrompt, time.Now().Format("Monday, January 2, 2006 3:04 PM")),
		},
	}

	for _, msg := range thread {
		role := openai.ChatMessageRoleUser
		if msg.ViaBot != nil {
			role = openai.ChatMessageRoleAssistant
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Text,
		})
	}

	stream, err := a.openai.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			Stream:   true,
		},
	)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk)
	go func() {
		defer stream.Close()
		defer close(ch)

		for {
			res, err := stream.Recv()
			if err != nil {
				ch <- StreamChunk{Error: err}
				return
			}
			ch <- StreamChunk{Delta: res.Choices[0].Delta.Content}
		}
	}()

	return ch, nil
}
