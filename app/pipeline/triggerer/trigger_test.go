package triggerer

import (
	"strings"
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

func TestValidateV1StageApproval(t *testing.T) {
	t.Parallel()

	tooManyApprovers := make([]string, 0, constant.MaxStageApprovalApprovers+1)
	for i := 0; i <= constant.MaxStageApprovalApprovers; i++ {
		tooManyApprovers = append(tooManyApprovers, "zhangsan")
	}

	// 审批人数量在限制内，但账号足够长，超过 approvalConfig 的列宽
	longApprovers := make([]string, 0, constant.MaxStageApprovalApprovers)
	for i := 0; i < constant.MaxStageApprovalApprovers; i++ {
		longApprovers = append(longApprovers, strings.Repeat("a", 25))
	}

	cases := []struct {
		name     string
		approval *v1yaml.Approval
		wantErr  string
	}{
		{name: "overall omitted", approval: nil},
		{name: "valid", approval: &v1yaml.Approval{
			Approvers: v1yaml.CommaSeparated{"zhangsan", "lisi"},
			Mode:      string(constant.StageApprovalModeAnd),
			Timeout:   "24h",
		}},
		{name: "mode defaulted", approval: &v1yaml.Approval{
			Approvers: v1yaml.CommaSeparated{"zhangsan"},
		}},
		{name: "empty approvers", approval: &v1yaml.Approval{}, wantErr: "check 'approvers' key"},
		{name: "too many approvers", approval: &v1yaml.Approval{Approvers: tooManyApprovers}, wantErr: "too many approvers"},
		{name: "invalid mode", approval: &v1yaml.Approval{
			Approvers: v1yaml.CommaSeparated{"zhangsan"},
			Mode:      "xor",
		}, wantErr: "invalid approval mode"},
		{name: "invalid timeout", approval: &v1yaml.Approval{
			Approvers: v1yaml.CommaSeparated{"zhangsan"},
			Timeout:   "24",
		}, wantErr: "invalid approval timeout"},
		{name: "non positive timeout", approval: &v1yaml.Approval{
			Approvers: v1yaml.CommaSeparated{"zhangsan"},
			Timeout:   "0s",
		}, wantErr: "invalid approval timeout"},
		{name: "too long config", approval: &v1yaml.Approval{Approvers: longApprovers}, wantErr: "is too long"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			err := validateV1StageApproval(&v1yaml.Stage{Id: "stage-1", Approval: c.approval})
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("expected error containing %q, got %q", c.wantErr, err.Error())
			}
		})
	}
}
