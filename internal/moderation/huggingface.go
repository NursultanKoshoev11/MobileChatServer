package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const defaultHuggingFaceEndpoint = "https://api-inference.huggingface.co/models"

type HuggingFaceConfig struct {
	Token     string
	Model     string
	Endpoint  string
	Threshold float64
}

type HuggingFaceClient struct {
	cfg        HuggingFaceConfig
	httpClient *http.Client
}

type huggingFaceRequest struct {
	Inputs     string         `json:"inputs"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Options    map[string]any `json:"options,omitempty"`
}

type huggingFaceLabel struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

func (c *HuggingFaceClient) Moderate(ctx context.Context, input Input) (Decision, error) {
	text := strings.TrimSpace(input.Title + "\n" + input.Body)
	if text == "" {
		return NewDecision(ActionAllow, "huggingface"), nil
	}
	payload, err := json.Marshal(huggingFaceRequest{
		Inputs: text,
		Parameters: map[string]any{
			"function_to_apply": "sigmoid",
			"top_k":             8,
		},
		Options: map[string]any{"wait_for_model": true},
	})
	if err != nil {
		return Decision{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return Decision{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Decision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Decision{}, fmt.Errorf("huggingface moderation status: %d", resp.StatusCode)
	}
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Decision{}, err
	}
	labels, err := parseHuggingFaceLabels(raw)
	if err != nil {
		return Decision{}, err
	}
	if len(labels) == 0 {
		return NewDecision(ActionAllow, "huggingface"), nil
	}

	threshold := c.cfg.Threshold
	if threshold <= 0 {
		threshold = 0.72
	}
	scoresJSON := "{}"
	if encoded, err := json.Marshal(labels); err == nil {
		scoresJSON = string(encoded)
	}
	for _, label := range labels {
		if !isRiskyHuggingFaceLabel(label.Label) || label.Score < threshold {
			continue
		}
		reason := fmt.Sprintf("huggingface:%s", strings.ToLower(label.Label))
		if label.Score >= 0.90 {
			return Decision{Action: ActionBlock, Provider: "huggingface", ProviderModel: c.cfg.Model, ProviderScoresJSON: scoresJSON, Reasons: []string{reason}}, nil
		}
		return Decision{Action: ActionReview, Provider: "huggingface", ProviderModel: c.cfg.Model, ProviderScoresJSON: scoresJSON, Reasons: []string{reason}}, nil
	}
	return Decision{Action: ActionAllow, Provider: "huggingface", ProviderModel: c.cfg.Model, ProviderScoresJSON: scoresJSON}, nil
}

func (c *HuggingFaceClient) endpoint() string {
	endpoint := strings.TrimRight(strings.TrimSpace(c.cfg.Endpoint), "/")
	if endpoint == "" {
		endpoint = defaultHuggingFaceEndpoint
	}
	if strings.Contains(endpoint, "{model}") {
		return strings.ReplaceAll(endpoint, "{model}", escapeModelPath(c.cfg.Model))
	}
	return endpoint + "/" + escapeModelPath(c.cfg.Model)
}

func escapeModelPath(model string) string {
	parts := strings.Split(strings.Trim(model, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func parseHuggingFaceLabels(raw json.RawMessage) ([]huggingFaceLabel, error) {
	var flat []huggingFaceLabel
	if err := json.Unmarshal(raw, &flat); err == nil && len(flat) > 0 {
		return flat, nil
	}
	var nested [][]huggingFaceLabel
	if err := json.Unmarshal(raw, &nested); err == nil && len(nested) > 0 {
		return nested[0], nil
	}
	var apiError struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &apiError); err == nil && apiError.Error != "" {
		return nil, fmt.Errorf("huggingface moderation error: %s", apiError.Error)
	}
	return nil, nil
}

func isRiskyHuggingFaceLabel(label string) bool {
	label = strings.ToLower(strings.TrimSpace(label))
	if strings.Contains(label, "non") || strings.Contains(label, "neutral") || strings.Contains(label, "safe") {
		return false
	}
	risky := []string{"toxic", "insult", "threat", "obscene", "hate", "abuse", "offensive", "label_1"}
	for _, fragment := range risky {
		if strings.Contains(label, fragment) {
			return true
		}
	}
	return false
}
