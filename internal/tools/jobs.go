package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxBgJobs   = 4
	maxBgOutput = 80 * 1024
	maxPollWait = 30
)

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *lockedBuf) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.b.Len() >= maxBgOutput {
		return len(p), nil
	}
	remain := maxBgOutput - w.b.Len()
	if len(p) > remain {
		p = p[:remain]
	}
	return w.b.Write(p)
}

func (w *lockedBuf) snapshot() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

type bgJob struct {
	id   int
	cmd  *exec.Cmd
	out  lockedBuf
	done chan struct{}
	mu   sync.Mutex
	err  error
}

func (j *bgJob) finished() bool {
	select {
	case <-j.done:
		return true
	default:
		return false
	}
}

func (j *bgJob) headline() string {
	state := "running"
	if j.finished() {
		j.mu.Lock()
		err := j.err
		j.mu.Unlock()
		if err != nil {
			state = "exited (" + err.Error() + ")"
		} else {
			state = "exited 0"
		}
	}
	pid := 0
	if j.cmd != nil && j.cmd.Process != nil {
		pid = j.cmd.Process.Pid
	}
	return fmt.Sprintf("job %d pid %d %s", j.id, pid, state)
}

func (j *bgJob) status() string {
	head := j.headline()
	out := strings.TrimRight(j.out.snapshot(), "\n\r")
	if out == "" {
		return head
	}
	return head + "\n" + out
}

type jobHub struct {
	mu   sync.Mutex
	next int
	jobs map[int]*bgJob
}

func newJobHub() *jobHub {
	return &jobHub{jobs: map[int]*bgJob{}}
}

func (h *jobHub) start(cmd *exec.Cmd, after func() error) (int, error) {
	if h == nil {
		return 0, fmt.Errorf("background jobs unavailable")
	}
	j := &bgJob{done: make(chan struct{})}
	cmd.Stdout = &j.out
	cmd.Stderr = &j.out
	h.mu.Lock()
	if h.runningLocked() >= maxBgJobs {
		h.mu.Unlock()
		return 0, fmt.Errorf("already %d background jobs; bash_kill one first", maxBgJobs)
	}
	if err := cmd.Start(); err != nil {
		h.mu.Unlock()
		return 0, err
	}
	h.next++
	j.id = h.next
	j.cmd = cmd
	h.jobs[j.id] = j
	h.mu.Unlock()
	go func() {
		err := cmd.Wait()
		if after != nil {
			if aerr := after(); aerr != nil {
				_, _ = fmt.Fprintf(&j.out, "\n(%v)", aerr)
				if err == nil {
					err = aerr
				}
			}
		}
		j.mu.Lock()
		j.err = err
		j.mu.Unlock()
		close(j.done)
	}()
	return j.id, nil
}

func (h *jobHub) runningLocked() int {
	n := 0
	for _, j := range h.jobs {
		if !j.finished() {
			n++
		}
	}
	return n
}

func (h *jobHub) get(id int) (*bgJob, error) {
	if h == nil {
		return nil, fmt.Errorf("unknown job %d", id)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	j, ok := h.jobs[id]
	if !ok {
		return nil, fmt.Errorf("unknown job %d", id)
	}
	return j, nil
}

func (h *jobHub) list() string {
	if h == nil {
		return "(no jobs)"
	}
	h.mu.Lock()
	ids := make([]int, 0, len(h.jobs))
	for id := range h.jobs {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	if len(ids) == 0 {
		return "(no jobs)"
	}
	sort.Ints(ids)
	var b bytes.Buffer
	for i, id := range ids {
		j, err := h.get(id)
		if err != nil {
			continue
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(j.status())
	}
	if b.Len() == 0 {
		return "(no jobs)"
	}
	return b.String()
}

func (h *jobHub) poll(id, waitSec int) (string, error) {
	if id <= 0 {
		return h.list(), nil
	}
	j, err := h.get(id)
	if err != nil {
		return "", err
	}
	if waitSec > maxPollWait {
		waitSec = maxPollWait
	}
	if waitSec > 0 && !j.finished() {
		timer := time.NewTimer(time.Duration(waitSec) * time.Second)
		select {
		case <-j.done:
			timer.Stop()
		case <-timer.C:
		}
	}
	return j.status(), nil
}

func (h *jobHub) kill(id int) (string, error) {
	j, err := h.get(id)
	if err != nil {
		return "", err
	}
	if !j.finished() {
		killGroup(j.cmd)
		<-j.done
	}
	return j.status(), nil
}

func (h *jobHub) summary() string {
	if h == nil {
		return "(no jobs)"
	}
	h.mu.Lock()
	ids := make([]int, 0, len(h.jobs))
	for id := range h.jobs {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	if len(ids) == 0 {
		return "(no jobs)"
	}
	sort.Ints(ids)
	var b bytes.Buffer
	for i, id := range ids {
		j, err := h.get(id)
		if err != nil {
			continue
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(j.headline())
	}
	if b.Len() == 0 {
		return "(no jobs)"
	}
	return b.String()
}

func (h *jobHub) counts() (running, total int) {
	if h == nil {
		return 0, 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	total = len(h.jobs)
	for _, j := range h.jobs {
		if !j.finished() {
			running++
		}
	}
	return running, total
}

func (h *jobHub) killAll() {
	if h == nil {
		return
	}
	h.mu.Lock()
	jobs := make([]*bgJob, 0, len(h.jobs))
	for _, j := range h.jobs {
		jobs = append(jobs, j)
	}
	h.jobs = map[int]*bgJob{}
	h.mu.Unlock()
	for _, j := range jobs {
		if !j.finished() {
			killGroup(j.cmd)
			<-j.done
		}
	}
}
