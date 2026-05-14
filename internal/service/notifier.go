package service

import "context"

type PushMessage struct {
	Title string
	Body  string
	Data  map[string]string
}

type Notifier interface {
	SendToTokens(ctx context.Context, tokens []string, message PushMessage) error
}

type NoopNotifier struct{}

func (NoopNotifier) SendToTokens(ctx context.Context, tokens []string, message PushMessage) error {
	return nil
}
