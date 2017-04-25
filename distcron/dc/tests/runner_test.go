package dc

import (
	"testing"

	"distcron/dc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	st, err := r.StopJob(ctx, cid)
	assert.NoError(t, err)
	t.Log(st)

	r.GetJobOutput(ctx, cid, print(t))
	assert.NoError(t, err)

	_, err = r.StopJob(ctx, "no-such-container")
	if assert.Error(t, err) {
		assert.Equal(t, dc.InternalError, err)
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
