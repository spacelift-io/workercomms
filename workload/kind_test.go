package workload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkloadKind_EnumValues pins the values stored in the workers.workload_kind column.
// Changing one requires migrating that column.
func TestWorkloadKind_EnumValues(t *testing.T) {
	assert.Len(t, workloadKindNames, 3, "Make sure to update this test if you add a new workload kind")
	cases := map[WorkloadKind]int{
		WorkloadKindUnknown: 0,
		WorkloadKindIntent:  1,
		WorkloadKindScan:    2,
		WorkloadKindRun:     3,
	}
	for kind, want := range cases {
		assert.EqualValues(t, want, kind)
	}
}

func TestWorkloadKind_String(t *testing.T) {
	cases := map[WorkloadKind]string{
		0:                "UNKNOWN",
		1:                "INTENT",
		2:                "SCAN",
		3:                "RUN",
		WorkloadKind(99): "UNKNOWN",
	}
	for kind, want := range cases {
		assert.Equal(t, want, kind.String())
	}
}

func TestParseWorkloadKind(t *testing.T) {
	assert.Len(t, workloadKindNames, 3, "Make sure to update this test if you add a new workload kind")
	cases := map[string]WorkloadKind{
		"RUN":    3,
		"INTENT": 1,
		"SCAN":   2,
		"run":    3,
		"Intent": 1,
	}
	for input, want := range cases {
		got, err := ParseWorkloadKind(input)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

func TestParseWorkloadKind_Unknown(t *testing.T) {
	for _, input := range []string{"", "UNKNOWN", "foobar"} {
		got, err := ParseWorkloadKind(input)
		require.Error(t, err)
		assert.Equal(t, WorkloadKindUnknown, got)
	}
}
