package sms

import (
	"context"
	"fmt"
	"log"
)

type Sender interface {
	SendVerificationCode(ctx context.Context, phoneNumber string, code string) error
}

type DevSender struct {
	Logger *log.Logger
}

func (s DevSender) SendVerificationCode(ctx context.Context, phoneNumber string, code string) error {
	if s.Logger != nil {
		s.Logger.Printf("dev sms verification phone=%s code=%s", phoneNumber, code)
	}
	return nil
}

type DisabledSender struct{}

func (DisabledSender) SendVerificationCode(ctx context.Context, phoneNumber string, code string) error {
	return fmt.Errorf("sms sender is not configured")
}
