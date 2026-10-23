package workload

import (
	"strings"

	"github.com/pkg/errors"
)

// WorkloadKind identifies the kind of workload a worker runs.
// This can be used to differentiate between regular spacelift runs, intent tasks and scan runs.
// In the launcher, especially for k8s workers, we might not exactly perform the same tasks as part
// as the init container or the grpc-server bridge depending on the workload kind.
type WorkloadKind int

// These values are stored in the workers.workload_kind column, so they cannot be
// renumbered without migrating that column.
const (
	WorkloadKindUnknown WorkloadKind = 0
	WorkloadKindIntent  WorkloadKind = 1
	WorkloadKindScan    WorkloadKind = 2
	WorkloadKindRun     WorkloadKind = 3
)

//exhaustive:enforce
var workloadKindNames = map[WorkloadKind]string{
	WorkloadKindRun:    "RUN",
	WorkloadKindIntent: "INTENT",
	WorkloadKindScan:   "SCAN",
}

func (r WorkloadKind) String() string {
	if t, ok := workloadKindNames[r]; ok {
		return t
	}
	return "UNKNOWN"
}

func ParseWorkloadKind(s string) (WorkloadKind, error) {
	for k, v := range workloadKindNames {
		if strings.EqualFold(v, s) {
			return k, nil
		}
	}
	return WorkloadKindUnknown, errors.Errorf("workload kind %s unknown", s)
}
