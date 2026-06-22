package service

import "context"

type PushMessage struct {
	Title string
	Body  string
	Data  map[string]string
}

type PushSendResult struct {
	InvalidTokens []string
}

type Notifier interface {
	SendToTokens(ctx context.Context, tokens []string, message PushMessage) (PushSendResult, error)
}

type NoopNotifier struct{}

func (NoopNotifier) SendToTokens(ctx context.Context, tokens []string, message PushMessage) (PushSendResult, error) {
	return PushSendResult{}, nil
}
