package storage

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (r *Repository) GetPublicRequestStatistics(ctx context.Context, groupID, viewerID, period, granularity string, from, to time.Time) (domain.PublicRequestStatistics, error) {
	isMember, err := r.IsGroupMember(ctx, groupID, viewerID)
	if err != nil {
		return domain.PublicRequestStatistics{}, err
	}
	if !isMember {
		return domain.PublicRequestStatistics{}, ErrForbidden
	}

	stats := domain.PublicRequestStatistics{
		GroupID:      groupID,
		Period:       period,
		Granularity:  granularity,
		From:         from.UTC().Format(time.RFC3339),
		To:           to.UTC().Format(time.RFC3339),
		ByType:       make([]domain.StatisticsBreakdownItem, 0),
		ByStatus:     make([]domain.StatisticsBreakdownItem, 0),
		Timeline:     make([]domain.StatisticsTimelineItem, 0),
		RecentOpenRequests: make([]domain.PublicRequest, 0),
	}

	if err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE request_type = 'complaint')::int,
			COUNT(*) FILTER (WHERE status IN ('resolved', 'rejected'))::int,
			COUNT(*) FILTER (WHERE status NOT IN ('resolved', 'rejected'))::int,
			COUNT(*) FILTER (WHERE status = 'resolved')::int,
			COUNT(*) FILTER (WHERE status = 'rejected')::int
		FROM public_requests
		WHERE group_id = $1 AND hidden_at IS NULL AND created_at >= $2 AND created_at < $3`, groupID, from, to).Scan(
		&stats.TotalRequests,
		&stats.TotalComplaints,
		&stats.ClosedRequests,
		&stats.OpenRequests,
		&stats.ResolvedRequests,
		&stats.RejectedRequests,
	); err != nil {
		return domain.PublicRequestStatistics{}, fmt.Errorf("load statistics totals: %w", err)
	}

	stats.CloseRate = percentage(stats.ClosedRequests, stats.TotalRequests)
	stats.ResolveRate = percentage(stats.ResolvedRequests, stats.TotalRequests)

	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM public_request_comments c
		JOIN public_requests pr ON pr.id = c.request_id
		WHERE pr.group_id = $1 AND pr.hidden_at IS NULL AND c.deleted_at IS NULL AND c.created_at >= $2 AND c.created_at < $3`, groupID, from, to).Scan(&stats.TotalComments); err != nil {
		return domain.PublicRequestStatistics{}, fmt.Errorf("load statistics comments: %w", err)
	}

	if err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE v.vote_type = 'support')::int,
			COUNT(*) FILTER (WHERE v.vote_type = 'oppose')::int
		FROM public_request_votes v
		JOIN public_requests pr ON pr.id = v.request_id
		WHERE pr.group_id = $1 AND pr.hidden_at IS NULL AND v.created_at >= $2 AND v.created_at < $3`, groupID, from, to).Scan(&stats.SupportVotes, &stats.OpposeVotes); err != nil {
		return domain.PublicRequestStatistics{}, fmt.Errorf("load statistics votes: %w", err)
	}

	byType, err := r.statisticsBreakdown(ctx, `request_type`, groupID, from, to, stats.TotalRequests)
	if err != nil {
		return domain.PublicRequestStatistics{}, err
	}
	stats.ByType = byType

	byStatus, err := r.statisticsBreakdown(ctx, `status`, groupID, from, to, stats.TotalRequests)
	if err != nil {
		return domain.PublicRequestStatistics{}, err
	}
	stats.ByStatus = byStatus

	byMode, err := r.statisticsBreakdown(ctx, `interaction_mode`, groupID, from, to, stats.TotalRequests)
	if err != nil {
		return domain.PublicRequestStatistics{}, err
	}
	stats.ByInteractionMode = byMode

	timeline, err := r.statisticsTimeline(ctx, groupID, from, to, granularity)
	if err != nil {
		return domain.PublicRequestStatistics{}, err
	}
	stats.Timeline = timeline

	recent, err := r.recentOpenPublicRequests(ctx, groupID, from, to)
	if err != nil {
		return domain.PublicRequestStatistics{}, err
	}
	stats.RecentOpenRequests = recent

	return stats, nil
}

func (r *Repository) statisticsBreakdown(ctx context.Context, column string, groupID string, from, to time.Time, total int) ([]domain.StatisticsBreakdownItem, error) {
	query := fmt.Sprintf(`
		SELECT %s::text AS key, COUNT(*)::int
		FROM public_requests
		WHERE group_id = $1 AND hidden_at IS NULL AND created_at >= $2 AND created_at < $3
		GROUP BY %s
		ORDER BY COUNT(*) DESC, key ASC`, column, column)
	rows, err := r.db.Query(ctx, query, groupID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load statistics breakdown %s: %w", column, err)
	}
	defer rows.Close()

	items := make([]domain.StatisticsBreakdownItem, 0)
	for rows.Next() {
		var item domain.StatisticsBreakdownItem
		if err := rows.Scan(&item.Key, &item.Count); err != nil {
			return nil, fmt.Errorf("scan statistics breakdown %s: %w", column, err)
		}
		item.Label = item.Key
		item.Percent = percentage(item.Count, total)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) statisticsTimeline(ctx context.Context, groupID string, from, to time.Time, granularity string) ([]domain.StatisticsTimelineItem, error) {
	dateTrunc := "day"
	if granularity == "week" || granularity == "month" || granularity == "year" {
		dateTrunc = granularity
	}
	query := fmt.Sprintf(`
		SELECT to_char(date_trunc('%s', created_at), 'YYYY-MM-DD') AS bucket,
			COUNT(*)::int AS total,
			COUNT(*) FILTER (WHERE status IN ('resolved', 'rejected'))::int AS closed,
			COUNT(*) FILTER (WHERE status NOT IN ('resolved', 'rejected'))::int AS open,
			COUNT(*) FILTER (WHERE status = 'resolved')::int AS resolved,
			COUNT(*) FILTER (WHERE request_type = 'complaint')::int AS complaints
		FROM public_requests
		WHERE group_id = $1 AND hidden_at IS NULL AND created_at >= $2 AND created_at < $3
		GROUP BY date_trunc('%s', created_at)
		ORDER BY date_trunc('%s', created_at) ASC`, dateTrunc, dateTrunc, dateTrunc)
	rows, err := r.db.Query(ctx, query, groupID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load statistics timeline: %w", err)
	}
	defer rows.Close()

	items := make([]domain.StatisticsTimelineItem, 0)
	for rows.Next() {
		var item domain.StatisticsTimelineItem
		if err := rows.Scan(&item.Bucket, &item.Total, &item.Closed, &item.Open, &item.Resolved, &item.Complaints); err != nil {
			return nil, fmt.Errorf("scan statistics timeline: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) recentOpenPublicRequests(ctx context.Context, groupID string, from, to time.Time) ([]domain.PublicRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pr.id, pr.group_id, pr.author_id, u.display_name, pr.request_type, pr.interaction_mode, pr.title, pr.body, pr.status,
			0::int AS support_count,
			0::int AS oppose_count,
			(SELECT COUNT(*)::int FROM public_request_comments c WHERE c.request_id = pr.id AND c.deleted_at IS NULL) AS comment_count,
			pr.created_at, pr.updated_at
		FROM public_requests pr
		JOIN users u ON u.id = pr.author_id
		WHERE pr.group_id = $1 AND pr.hidden_at IS NULL AND pr.created_at >= $2 AND pr.created_at < $3 AND pr.status NOT IN ('resolved', 'rejected')
		ORDER BY pr.created_at DESC
		LIMIT 10`, groupID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load recent open requests: %w", err)
	}
	defer rows.Close()

	requests := make([]domain.PublicRequest, 0)
	for rows.Next() {
		var request domain.PublicRequest
		if err := rows.Scan(&request.ID, &request.GroupID, &request.AuthorID, &request.AuthorName, &request.RequestType, &request.InteractionMode, &request.Title, &request.Body, &request.Status, &request.SupportCount, &request.OpposeCount, &request.CommentCount, &request.CreatedAt, &request.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan recent open request: %w", err)
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func percentage(value, total int) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round((float64(value) / float64(total) * 100) * 10) / 10
}
