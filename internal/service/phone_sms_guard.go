package service

import (
	"context"
	"log"
	"strings"
	"time"
)

// smsCountryAllowed reports whether a verification SMS may be sent to mobile
// (already normalized to +<country><number>).
func (s *PhoneAuthService) smsCountryAllowed(mobile string) bool {
	prefixes := s.cfg.SMSAllowedCountryPrefixes
	if len(prefixes) == 0 {
		prefixes = defaultSMSAllowedCountryPrefixes
	}
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && strings.HasPrefix(mobile, prefix) {
			return true
		}
	}
	return false
}

func (s *PhoneAuthService) smsGlobalHourlyMax() int {
	if s.cfg.SMSGlobalHourlyMax > 0 {
		return s.cfg.SMSGlobalHourlyMax
	}
	return defaultPhoneCodeGlobalHourlyMax
}

// checkSMSBudget enforces a per-number daily cap (stops SMS bombing of one
// person) and a global hourly budget (stops SMS pumping across many numbers).
// When the global budget trips, sending stops for everyone until the window
// passes; that is logged loudly so operators can react.
func (s *PhoneAuthService) checkSMSBudget(ctx context.Context, mobile string, now time.Time) error {
	phoneCount, globalCount, err := s.repo.CountPhoneCodesIssued(ctx, mobile, now.Add(-phoneCodeDailyWindow), now.Add(-phoneCodeGlobalWindow))
	if err != nil {
		return err
	}
	if phoneCount >= phoneCodeDailyMax {
		return NewValidationError("too many code requests; try again later")
	}
	if limit := s.smsGlobalHourlyMax(); globalCount >= limit {
		log.Printf("ALERT sms_global_budget_exceeded issued_last_hour=%d limit=%d: verification SMS paused", globalCount, limit)
		return NewServiceUnavailableError("sms verification is temporarily unavailable")
	}
	return nil
}
