package sms

import (
	"context"
	"errors"
	"log"
)

type Sender interface {
	SendVerificationCode(ctx context.Context, phoneNumber string, code string) error
}

var ErrDisabled = errors.New("sms sender is disabled")

type DevSender struct {
	Logger *log.Logger
}

func (s DevSender) SendVerificationCode(ctx context.Context, phoneNumber string, code string) error {
	if s.Logger != nil {
		s.Logger.Printf("dev sms verification phone=%s code=[REDACTED]", phoneNumber)
	}
	return nil
}

type DisabledSender struct{}

func (DisabledSender) SendVerificationCode(ctx context.Context, phoneNumber string, code string) error {
	return ErrDisabled
}
