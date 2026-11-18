package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func TestValidateCreateReportInput(t *testing.T) {
	valid := CreateReportInput{TargetType: "comment", TargetID: " COM-1 ", Reason: "spam", Details: " text "}
	got, err := ValidateCreateReportInput(valid)
	if err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if got.TargetID != "COM-1" || got.Details != "text" {
		t.Fatalf("input not normalized: %+v", got)
	}
	invalid := []CreateReportInput{
		{TargetType: "group", TargetID: "G-1", Reason: "spam"},
		{TargetType: "comment", TargetID: "", Reason: "spam"},
		{TargetType: "comment", TargetID: "COM-1", Reason: "rude"},
		{TargetType: "comment", TargetID: "COM-1", Reason: "other", Details: strings.Repeat("д", maxReportDetailsLen+1)},
	}
	for _, input := range invalid {
		var validation ValidationError
		if _, err := ValidateCreateReportInput(input); !errors.As(err, &validation) {
			t.Errorf("expected validation error for %+v, got %v", input, err)
		}
	}
	// 1000 multi-byte characters are within the limit.
	if _, err := ValidateCreateReportInput(CreateReportInput{TargetType: "user", TargetID: "U-1", Reason: "other", Details: strings.Repeat("д", maxReportDetailsLen)}); err != nil {
		t.Fatalf("1000 characters must be accepted: %v", err)
	}
}

func TestParseReportStatus(t *testing.T) {
	for raw, want := range map[string]domain.ReportStatus{"": domain.ReportStatusOpen, "open": domain.ReportStatusOpen, "resolved": domain.ReportStatusResolved} {
		got, err := ParseReportStatus(raw)
		if err != nil || got != want {
			t.Errorf("ParseReportStatus(%q) = %q, %v", raw, got, err)
		}
	}
	if _, err := ParseReportStatus("closed"); err == nil {
		t.Fatal("unknown status must be rejected")
	}
}

func TestSelfTargetedBlockAndReportAreRejected(t *testing.T) {
	svc := &Service{}
	var validation ValidationError
	if err := svc.BlockUser(context.Background(), "U-1", "U-1"); !errors.As(err, &validation) {
		t.Fatalf("self block: %v", err)
	}
	if _, _, err := svc.CreateReport(context.Background(), "U-1", CreateReportInput{TargetType: "user", TargetID: "U-1", Reason: "spam"}); !errors.As(err, &validation) {
		t.Fatalf("self report: %v", err)
	}
	if _, _, err := svc.ResolveReport(context.Background(), "U-1", "RPT-1", ResolveReportInput{Action: "ban"}); !errors.As(err, &validation) {
		t.Fatalf("invalid resolve action: %v", err)
	}
}
