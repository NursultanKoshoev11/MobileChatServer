package service

import (
	"context"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

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
	granularity := normalizeStatisticsGranularity(input.Granularity)
	from, to := statisticsRange(period, input.From, input.To)

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

func normalizeStatisticsGranularity(granularity string) string {
	granularity = strings.ToLower(strings.TrimSpace(granularity))
	switch granularity {
	case "day", "week", "month", "year":
		return granularity
	default:
		return "day"
	}
}

func statisticsRange(period string, customFrom, customTo time.Time) (time.Time, time.Time) {
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
		if !customFrom.IsZero() {
			from = customFrom.UTC()
		} else {
			from = now.AddDate(0, -1, 0)
		}
		if !customTo.IsZero() {
			to = customTo.UTC()
		}
	default:
		from = now.AddDate(0, -1, 0)
	}

	if !from.Before(to) {
		from = to.AddDate(0, -1, 0)
	}
	return from, to
}
