package internal

import (
	"context"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openai "github.com/sashabaranov/go-openai"
)

type AI struct {
	openai *openai.Client
}

func NewAI() *AI {
	return &AI{openai.NewClient(MustGetEnv("ROBOTRON_OPENAI_API_KEY"))}
}

type StreamChunk struct {
	Delta string
	Error error
}

func (a *AI) StreamingReply(ctx context.Context, thread []*telegram.Message) (chan StreamChunk, error) {
	messages := []openai.ChatCompletionMessage{}
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
