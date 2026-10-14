package repo

// AIReviewAcceptReq 一键应用 AI 审查建议请求体。
type AIReviewAcceptReq struct {
	IssueIDs []int64 `json:"issueIDs" binding:"required,min=1"`
}

// AIReviewAcceptItemStatus 单条 issue 应用结果状态。
type AIReviewAcceptItemStatus string

const (
	AIReviewAcceptStatusAccepted AIReviewAcceptItemStatus = "accepted" // 已应用
	AIReviewAcceptStatusSkipped  AIReviewAcceptItemStatus = "skipped"  // 已跳过（原因见 Message）
	AIReviewAcceptStatusFailed   AIReviewAcceptItemStatus = "failed"   // 应用失败（原因见 Message）
)

// AIReviewAcceptItem 单条 issue 的应用结果。
type AIReviewAcceptItem struct {
	IssueID   int64                    `json:"issueID"`
	Path      string                   `json:"path"`
	Status    AIReviewAcceptItemStatus `json:"status"`
	CommitSHA string                   `json:"commitSHA,omitempty"`
	Message   string                   `json:"message,omitempty"`
}

// AIReviewAcceptResponse 一键应用 AI 审查建议的响应。
type AIReviewAcceptResponse struct {
	Items []*AIReviewAcceptItem `json:"items"`
}
