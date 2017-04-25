package dc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestRunner(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	cid, err := r.RunJob(ctx, &Job{
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
		assert.Equal(t, InternalError, err)
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
