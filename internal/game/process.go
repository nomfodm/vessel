package game

import (
	"bufio"
	"io"
	"os/exec"
	"sync"
)

// runner starts a process. Injected so the service can be tested without Java.
type runner interface {
	Start(java string, args []string, dir string) (*process, error)
}

// process is a running child with merged stdout/stderr exposed line by line.
type process struct {
	cmd      *exec.Cmd
	lines    chan string
	waitErr  chan error
	exitCode int
}

func (p *process) Lines() <-chan string { return p.lines }

func (p *process) Wait() int {
	err := <-p.waitErr
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

func (p *process) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

type osRunner struct{}

func (osRunner) Start(java string, args []string, dir string) (*process, error) {
	cmd := exec.Command(java, args...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	p := &process{
		cmd:     cmd,
		lines:   make(chan string, 64),
		waitErr: make(chan error, 1),
	}

	// stdout and stderr are scanned concurrently so live output is interleaved,
	// not buffered until one stream closes.
	var wg sync.WaitGroup
	wg.Add(2)
	go p.scan(&wg, stdout)
	go p.scan(&wg, stderr)
	go func() {
		wg.Wait()
		close(p.lines)
		p.waitErr <- p.cmd.Wait()
	}()

	return p, nil
}

func (p *process) scan(wg *sync.WaitGroup, r io.Reader) {
	defer wg.Done()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		p.lines <- sc.Text()
	}
}
