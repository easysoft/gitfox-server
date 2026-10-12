package constant

// StageApprovalStatus stage 审批状态。取值与禅道 DevOps 侧约定一致。
type StageApprovalStatus string

const (
	StageApprovalStatusNone       StageApprovalStatus = ""           // 未发起（表默认值，创建审批的触发条件）
	StageApprovalStatusInProgress StageApprovalStatus = "inProgress" // 审批中
	StageApprovalStatusApproved   StageApprovalStatus = "approved"   // 通过
	StageApprovalStatusRejected   StageApprovalStatus = "rejected"   // 拒绝
	StageApprovalStatusCanceled   StageApprovalStatus = "canceled"   // 已取消
)

// StageApprovalMode 审批方式。
type StageApprovalMode string

const (
	StageApprovalModeAnd StageApprovalMode = "and" // 会签（默认）
	StageApprovalModeOr  StageApprovalMode = "or"  // 或签
)

const (
	// MaxApprovalConfigLen 与 ops_stage_executions.approvalConfig 的列宽保持一致
	MaxApprovalConfigLen = 500
	// MaxStageApprovalApprovers 单 stage 审批人上限
	MaxStageApprovalApprovers = 20
)
