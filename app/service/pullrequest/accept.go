package pullrequest

import (
	"context"
	"fmt"

	"github.com/easysoft/gitfox-community/app/model/auth"
	repoModel "github.com/easysoft/gitfox-community/app/model/repo"
)

// AIIssueApplier applies AI review suggestions onto a pull request source
// branch (one-click accept).
//
// The community edition binds NoopAIIssueApplier; the paid edition provides the
// implementation that rewrites the source-branch file by line range and commits
// the change.
type AIIssueApplier interface {
	AcceptAIIssue(ctx context.Context, session *auth.Session, repoID, pullreqID uint, req *repoModel.AIReviewAcceptReq) (*repoModel.AIReviewAcceptResponse, error)
}

// NoopAIIssueApplier is the community-edition implementation of AIIssueApplier.
type NoopAIIssueApplier struct{}

var _ AIIssueApplier = (*NoopAIIssueApplier)(nil)

// NewNoopAIIssueApplier creates the community-edition AIIssueApplier.
func NewNoopAIIssueApplier() *NoopAIIssueApplier {
	return &NoopAIIssueApplier{}
}

// AcceptAIIssue is unavailable in the community edition.
func (NoopAIIssueApplier) AcceptAIIssue(_ context.Context, _ *auth.Session, _, _ uint, _ *repoModel.AIReviewAcceptReq) (*repoModel.AIReviewAcceptResponse, error) {
	return nil, fmt.Errorf("ai issue apply is not available in the community edition")
}

// AcceptAIIssue delegates to the injected applier.
func (s *Service) AcceptAIIssue(ctx context.Context, session *auth.Session, repoID, pullreqID uint, req *repoModel.AIReviewAcceptReq) (*repoModel.AIReviewAcceptResponse, error) {
	return s.aiIssueApplier.AcceptAIIssue(ctx, session, repoID, pullreqID, req)
}
