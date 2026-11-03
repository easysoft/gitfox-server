package provider

import (
	"github.com/google/wire"

	"github.com/easysoft/gitfox-community/app/pipeline/scheduler"
)

// CommunityStageApprovalWireSet binds the community-edition no-op stage approval
// implementation. The paid edition replaces this with its own wire set.
var CommunityStageApprovalWireSet = wire.NewSet(
	wire.Bind(new(scheduler.IStageApproval), new(*scheduler.NoopStageApproval)),
	scheduler.NewNoopStageApproval,
)
