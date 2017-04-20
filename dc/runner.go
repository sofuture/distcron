package dc

import (
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type runner struct {
}

func NewRunner() Runner {
	return &runner{}
}

func (r *runner) RunJob(job *Job) (cid string, err error) {
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

func (r *runner) GetJobStatus(cid string) (status *JobStatus, err error) {
	cmd := exec.Command("docker", "ps", "--all",
		"--format", "{{.Status}}",
		"--filter", fmt.Sprintf("id=%s", cid))

	if out, err := cmd.Output(); err != nil {
		glog.Errorf("%v failed: %v", cmd, err)
		return nil, EInternalError
	} else {
		return &JobStatus{string(out)}, nil
	}
}

func (r *runner) StopJob(cid string) (status *JobStatus, err error) {
	cmd := exec.Command("docker", "stop", cid)
	if out, err := cmd.Output(); err != nil {
		glog.Error(err)
		return nil, EInternalError
	} else {
		return &JobStatus{string(out)}, nil
	}
}

func (r *runner) GetJobOutput(cid string, fn DataCopyFn) (err error) {
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

	data := make([]byte, cBufSize)

	forwardData := true
	for {
		n, err := stdout.Read(data)
		if err != nil {
			if err != io.EOF {
				glog.Error(err)
			}
			break
		}
		if forwardData {
			if err = fn(data[0:n]); err != nil {
				glog.Error(err)
				forwardData = false
			}
		}
	}

	if err = cmd.Wait(); err != nil {
		glog.Error(err)
		return err
	}

	return nil
}
