package service

import (
	"regexp"
	"strings"
)

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

type RequestPhoneCodeInput struct {
	Mobile string `json:"mobile"`
}

type VerifyPhoneCodeInput struct {
	Mobile      string `json:"mobile"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
}

type RequestPhoneCodeOutput struct {
	Status        string `json:"status"`
	DevCode       string `json:"dev_code,omitempty"`
	AccountExists bool   `json:"account_exists"`
}

func normalizeMobile(raw string) (string, error) {
	mobile := strings.ReplaceAll(strings.TrimSpace(raw), " ", "")
	mobile = strings.ReplaceAll(mobile, "-", "")
	mobile = strings.ReplaceAll(mobile, "(", "")
	mobile = strings.ReplaceAll(mobile, ")", "")
	if !phonePattern.MatchString(mobile) {
		return "", NewValidationError("mobile must be in international format, for example +996700123456")
	}
	return mobile, nil
}
