package shell

import (
	"bufio"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	logger logrus.FieldLogger

	terminated chan struct{}
}

func NewSession() (Session, error) {
	cmd := exec.Command("bash")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Session{}, errors.Wrap(err, "session.NewSession: stdin pipe")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Session{}, errors.Wrap(err, "session.NewSession: stdout pipe")
	}

	cmd.Stderr = cmd.Stdout

	return Session{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		logger:     logrus.WithField("component", "shell.Session"),
		terminated: make(chan struct{}),
	}, nil
}

func (s *Session) Start() error {
	wg := sync.WaitGroup{}
	var err error
	wg.Add(1)
	go func() {
		defer close(s.terminated)
		err = s.cmd.Start()
		wg.Done()
		if err != nil {
			return
		}

		s.cmd.Wait()
		s.logger.Infof("stopped session with PID %d", s.cmd.Process.Pid)
	}()
	wg.Wait()

	if err != nil {
		s.logger.WithError(err).Errorf("error starting process")
		return err
	}

	s.logger.Infof("started session with PID %d", s.cmd.Process.Pid)
	return nil
}

func (s *Session) Stop() error {
	if err := s.cmd.Process.Kill(); err != nil {
		return err
	}

	<-s.terminated
	return nil
}

func (s *Session) Alive() bool {
	select {
	case <-s.terminated:
		return false
	default:
		break
	}

	if s == nil || s.cmd == nil || s.cmd.ProcessState == nil {
		return true
	}

	return !s.cmd.ProcessState.Exited()
}

var ErrSessionTerminated = errors.New("shell.Session: session has been terminated")

func (s *Session) Do(cmd string) (string, error) {
	if !s.Alive() {
		return "", ErrSessionTerminated
	}

	// send the actual command
	if _, err := s.stdin.Write([]byte(cmd + "\n")); err != nil {
		return "", errors.Wrap(err, "shell.Session: cmd write")
	}

	// also throw out a line that will be \0
	if _, err := s.stdin.Write([]byte("echo -ne '\\0'" + "\n")); err != nil {
		return "", errors.Wrap(err, "shell.Session: delim write")
	}

	// read up until the null byte
	output, err := bufio.NewReader(s.stdout).ReadString('\000')
	if err != nil {
		return "", errors.Wrap(err, "shell.Session: read command result")
	}

	// trim off the null byte
	return strings.Trim(output, "\n\000"), nil
}
