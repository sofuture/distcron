package dc

import (
	"testing"
)

func TestRunner(t *testing.T) {
	r := NewRunner()

	cid, err := r.RunJob(&Job{
		ContainerName: "hello-world",
		CpuLimit:      1,
		MemLimitMb:    500,
	})
	if err != nil {
		t.Error(err)
		return
	}

	r.GetJobOutput(cid, print(t))

	if st, err := r.StopJob("no-such-container"); err == nil {
		t.Errorf("stopping inexistent container : %v", st)
	} else {
		t.Log(err)
	}
}

func print(t *testing.T) DataCopyFn {
	return func(data []byte) error {
		if len(data) > 0 {
			t.Log(string(data))
		}
		return nil
	}
}
