package service

import (
	"context"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

const maxStatisticsCustomRange = 370 * 24 * time.Hour

type PublicRequestStatisticsInput struct {
	Period      string
	Granularity string
	From        time.Time
	To          time.Time
}

func (s *Service) GetPublicRequestStatistics(ctx context.Context, viewerID, groupID string, input PublicRequestStatisticsInput) (domain.PublicRequestStatistics, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return domain.PublicRequestStatistics{}, NewValidationError("group_id is required")
	}

	period := normalizeStatisticsPeriod(input.Period)
	granularity := normalizeStatisticsGranularity(input.Granularity, period)
	from, to, err := statisticsRange(period, input.From, input.To)
	if err != nil {
		return domain.PublicRequestStatistics{}, err
	}

	return s.repo.GetPublicRequestStatistics(ctx, groupID, viewerID, period, granularity, from, to)
}

func normalizeStatisticsPeriod(period string) string {
	period = strings.ToLower(strings.TrimSpace(period))
	switch period {
	case "week", "month", "year", "all", "custom":
		return period
	default:
		return "month"
	}
}

func normalizeStatisticsGranularity(granularity, period string) string {
	granularity = strings.ToLower(strings.TrimSpace(granularity))
	switch granularity {
	case "day", "week", "month", "year":
		return granularity
	}
	if period == "year" || period == "all" {
		return "month"
	}
	return "day"
}

func statisticsRange(period string, customFrom, customTo time.Time) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	to := now.Add(time.Second)
	var from time.Time

	switch period {
	case "week":
		from = now.AddDate(0, 0, -7)
	case "year":
		from = now.AddDate(-1, 0, 0)
	case "all":
		from = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	case "custom":
		if customFrom.IsZero() || customTo.IsZero() {
			return time.Time{}, time.Time{}, NewValidationError("custom statistics period requires from and to")
		}
		from = customFrom.UTC()
		to = customTo.UTC()
		if !from.Before(to) {
			return time.Time{}, time.Time{}, NewValidationError("from must be before to")
		}
		if to.Sub(from) > maxStatisticsCustomRange {
			return time.Time{}, time.Time{}, NewValidationError("custom statistics period cannot be longer than 370 days")
		}
	default:
		from = now.AddDate(0, -1, 0)
	}

	if !from.Before(to) {
		return time.Time{}, time.Time{}, NewValidationError("statistics period is invalid")
	}
	return from, to, nil
}
