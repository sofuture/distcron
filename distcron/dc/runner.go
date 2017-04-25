package dc

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
)

type runner struct {
}

func NewRunner() Runner {
	return &runner{}
}

func (r *runner) RunJob(ctx context.Context, job *Job) (cid string, err error) {
	tmpDir, err := ioutil.TempDir("", "dcron")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	cidFile := filepath.Join(tmpDir, "cid")
	cmd := exec.Command("docker", "run",
		"--cidfile", cidFile,
		"--cpus", fmt.Sprintf("%.1f", job.CpuLimit),
		"--memory", fmt.Sprintf("%dm", job.MemLimitMb),
		job.ContainerName)

	if out, err := cmd.CombinedOutput(); err != nil {
		glog.Errorf("Failed to run cmd %v : %v", cmd, err)
		glog.Error(string(out))
		return "", err
	} else if cidBytes, err := ioutil.ReadFile(cidFile); err != nil {
		glog.Error(err)
		return "", err
	} else {
		return string(cidBytes), nil
	}
}

const cBufSize = 1024

// how much time to wait for cmd.Wait() completion
const cmdWaitTimeout = time.Second

// GetJobStatus won't parse Docker output and just return as is, thus every time status might be different for MVP purpose
func (r *runner) GetJobStatus(ctx context.Context, cid string) (status *JobStatus, err error) {
	cmd := exec.Command("docker", "ps", "--all",
		"--format", "{{.Status}}",
		"--filter", fmt.Sprintf("id=%s", cid))

	if out, err := cmd.Output(); err != nil {
		glog.Errorf("%v failed: %v", cmd, err)
		return nil, InternalError
	} else {
		return &JobStatus{string(out)}, nil
	}
}

func (r *runner) StopJob(ctx context.Context, cid string) (status *JobStatus, err error) {
	cmd := exec.Command("docker", "stop", cid)
	out, err := cmd.CombinedOutput()
	if err != nil {
		glog.Error(cid, err, string(out))
		return &JobStatus{string(out)}, InternalError
	}

	return r.GetJobStatus(ctx, cid)
}

func copyProcOutput(ctx context.Context, reader io.ReadCloser, fn DataCopyFn) error {
	data := make([]byte, cBufSize)

	for {
		select {
		case <-ctx.Done():
			reader.Close()
			return RequestTimeoutError
		default:
		}

		n, err := reader.Read(data)
		if err != nil {
			if err != io.EOF {
				glog.Error(err)
			}
			break
		}

		if err = fn(data[0:n]); err != nil {
			glog.Error(err)
			reader.Close()
			return err
		}
	}

	return nil
}

func (r *runner) GetJobOutput(ctx context.Context, cid string, fn DataCopyFn) (err error) {
	cmd := exec.Command("docker", "logs",
		"--follow", cid)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		glog.Error(err)
		return err
	}

	if err = copyProcOutput(ctx, stdout, fn); err != nil {
		glog.Error(err)
	}

	// cmd.Wait() may be waiting indefinitely on process completion
	// due to stdout not exhausted thus a workaround
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, cmdWaitTimeout)

	go func() {
		defer timeoutCancel()
		if err = cmd.Wait(); err != nil {
			glog.Error(err)
		}
	}()

	<-timeoutCtx.Done()
	if timeoutCtx.Err() == context.DeadlineExceeded {
		glog.Error("cmd.Wait timeout for %s", cid)
		err = RequestTimeoutError
	}
	return err
}
