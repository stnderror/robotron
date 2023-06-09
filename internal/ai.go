package internal

import (
	"context"

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

func (a *AI) StreamingReply(ctx context.Context, text string) (chan StreamChunk, error) {
	stream, err := a.openai.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
			Stream: true,
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
