package moderation

import (
	"context"
)

type ContentType string

const (
	ContentTypeGroupMessage         ContentType = "group_message"
	ContentTypePublicRequest        ContentType = "public_request"
	ContentTypePublicRequestComment ContentType = "public_request_comment"
)

type Input struct {
	ContentType ContentType
	GroupID     string
	AuthorID    string
	TargetID    string
	Title       string
	Body        string
}

type Action string

const (
	ActionAllow  Action = "allow"
	ActionReview Action = "review"
	ActionBlock  Action = "block"
)

type Decision struct {
	Action             Action
	Reasons            []string
	Provider           string
	ProviderModel      string
	ProviderResponseID string
	ProviderScoresJSON string
}

type Moderator interface {
	Moderate(ctx context.Context, input Input) (Decision, error)
}

func NewDecision(action Action, provider string, reasons ...string) Decision {
	return Decision{Action: action, Provider: provider, Reasons: reasons}
}
