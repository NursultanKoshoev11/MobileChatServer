package domain

// StatisticsBreakdownItem represents a count and percentage for one category.
type StatisticsBreakdownItem struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// StatisticsTimelineItem represents grouped activity for a selected period.
type StatisticsTimelineItem struct {
	Bucket     string `json:"bucket"`
	Total      int    `json:"total"`
	Closed     int    `json:"closed"`
	Open       int    `json:"open"`
	Resolved   int    `json:"resolved"`
	Complaints int    `json:"complaints"`
}

// PublicRequestStatistics is a public dashboard payload for one group.
type PublicRequestStatistics struct {
	GroupID        string                    `json:"group_id"`
	Period         string                    `json:"period"`
	Granularity    string                    `json:"granularity"`
	From           string                    `json:"from"`
	To             string                    `json:"to"`
	TotalRequests  int                       `json:"total_requests"`
	TotalComplaints int                      `json:"total_complaints"`
	TotalComments  int                       `json:"total_comments"`
	SupportVotes   int                       `json:"support_votes"`
	OpposeVotes    int                       `json:"oppose_votes"`
	ClosedRequests int                       `json:"closed_requests"`
	OpenRequests   int                       `json:"open_requests"`
	ResolvedRequests int                     `json:"resolved_requests"`
	RejectedRequests int                     `json:"rejected_requests"`
	CloseRate      float64                   `json:"close_rate"`
	ResolveRate    float64                   `json:"resolve_rate"`
	ByType         []StatisticsBreakdownItem `json:"by_type"`
	ByStatus       []StatisticsBreakdownItem `json:"by_status"`
	ByInteractionMode []StatisticsBreakdownItem `json:"by_interaction_mode"`
	Timeline       []StatisticsTimelineItem  `json:"timeline"`
	RecentOpenRequests []PublicRequest       `json:"recent_open_requests"`
}
