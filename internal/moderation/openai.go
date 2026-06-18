package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultOpenAIEndpoint = "https://api.openai.com/v1/moderations"

type OpenAIConfig struct {
	Enabled    bool
	APIKey     string
	Model      string
	Endpoint   string
	FailClosed bool
	Timeout    time.Duration
}

type CompositeModerator struct {
	rules  RuleChecker
	openai *OpenAIClient
	cfg    OpenAIConfig
}

func NewCompositeModerator(cfg OpenAIConfig) *CompositeModerator {
	if cfg.Model == "" {
		cfg.Model = "omni-moderation-latest"
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultOpenAIEndpoint
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	moderator := &CompositeModerator{rules: NewRuleChecker(), cfg: cfg}
	if cfg.Enabled && strings.TrimSpace(cfg.APIKey) != "" {
		moderator.openai = &OpenAIClient{cfg: cfg, httpClient: &http.Client{Timeout: cfg.Timeout}}
	}
	return moderator
}

func (m *CompositeModerator) Moderate(ctx context.Context, input Input) (Decision, error) {
	if !m.cfg.Enabled {
		return NewDecision(ActionAllow, "disabled"), nil
	}
	ruleDecision := m.rules.Check(input)
	if ruleDecision.Action != ActionAllow {
		return ruleDecision, nil
	}
	if m.openai == nil {
		return ruleDecision, nil
	}
	decision, err := m.openai.Moderate(ctx, input)
	if err != nil {
		if m.cfg.FailClosed {
			return NewDecision(ActionReview, "openai", "moderation_provider_error"), nil
		}
		return ruleDecision, nil
	}
	return decision, nil
}

type OpenAIClient struct {
	cfg        OpenAIConfig
	httpClient *http.Client
}

type moderationRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type moderationResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Results []struct {
		Flagged        bool               `json:"flagged"`
		Categories     map[string]bool    `json:"categories"`
		CategoryScores map[string]float64 `json:"category_scores"`
	} `json:"results"`
}

func (c *OpenAIClient) Moderate(ctx context.Context, input Input) (Decision, error) {
	text := strings.TrimSpace(input.Title + "\n" + input.Body)
	if text == "" {
		return NewDecision(ActionAllow, "openai"), nil
	}
	payload, err := json.Marshal(moderationRequest{Model: c.cfg.Model, Input: text})
	if err != nil {
		return Decision{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return Decision{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Decision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Decision{}, fmt.Errorf("openai moderation status: %d", resp.StatusCode)
	}
	var parsed moderationResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Decision{}, err
	}
	if len(parsed.Results) == 0 {
		return Decision{}, fmt.Errorf("openai moderation returned no results")
	}
	result := parsed.Results[0]
	scoresJSON := "{}"
	if encoded, err := json.Marshal(result.CategoryScores); err == nil {
		scoresJSON = string(encoded)
	}
	reasons := make([]string, 0)
	for category, flagged := range result.Categories {
		if flagged {
			reasons = append(reasons, "openai:"+category)
		}
	}
	if result.Flagged {
		if len(reasons) == 0 {
			reasons = append(reasons, "openai:flagged")
		}
		return Decision{Action: ActionBlock, Provider: "openai", ProviderModel: parsed.Model, ProviderResponseID: parsed.ID, ProviderScoresJSON: scoresJSON, Reasons: reasons}, nil
	}
	return Decision{Action: ActionAllow, Provider: "openai", ProviderModel: parsed.Model, ProviderResponseID: parsed.ID, ProviderScoresJSON: scoresJSON}, nil
}
