package dc

import (
	"testing"

	"distcron/dc"

	"golang.org/x/net/context"
)

func TestRunner(t *testing.T) {
	r := dc.NewRunner()
	ctx := context.Background()

	cid, err := r.RunJob(ctx, &dc.Job{
		ContainerName: "hello-world",
		CpuLimit:      1,
		MemLimitMb:    500,
	})
	if err != nil {
		t.Error(err)
		return
	}

	r.GetJobOutput(ctx, cid, print(t))

	if st, err := r.StopJob(ctx, "no-such-container"); err == nil {
		t.Errorf("stopping inexistent container : %v", st)
	} else {
		t.Log(err)
	}
}

func print(t *testing.T) dc.DataCopyFn {
	return func(data []byte) error {
		if len(data) > 0 {
			t.Log(string(data))
		}
		return nil
	}
}
