package probe_status

import (
	"os"
	"testing"

	probev1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func initEvn() {
	_ = os.Setenv(probev1.ProbeNamespace, "default")
	_ = os.Setenv(probev1.ProbeName, "probe-link-test1")
	_ = os.Setenv(probev1.ProbeItemName, "probe-item-test2")
	_ = os.Setenv(probev1.ProbeStatusReportUrl, "http://localhost:8081/probe-status")
}

func TestReportStatus(t *testing.T) {
	initEvn()
	now := metav1.Now()
	checker1 := probev1.ProbeCheckerStatus{
		Name:    "probe-checker-test1",
		Status:  probev1.CheckerStatusInfo,
		Message: "probe-checker-test1 error",
		LastRun: nil,
	}
	checker2 := probev1.ProbeCheckerStatus{
		Name:    "probe-checker-test2",
		Status:  probev1.CheckerStatusInfo,
		Message: "probe-checker-test2 info",
		LastRun: &now,
	}
	checkers := []probev1.ProbeCheckerStatus{checker1, checker2}
	err := ReportProbeStatus(checkers)
	assert.NoError(t, err)
}
