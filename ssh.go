package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/term"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
)

type SSH struct {
	standup  *StandUp
	config   *ssh.ClientConfig
	session  *ssh.Session
	tempFile string
}

func NewSSH(standup *StandUp) (*SSH, error) {
	s := &SSH{
		standup: standup,
	}

	if err := s.readPubKey(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SSH) vimStandup(nowPath, yestPath string) error {
	c, err := s.loadComment(yestPath)
	if err != nil {
		return err
	}

	b, err := s.readStandupFile(nowPath)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/tmp/%s", filepath.Base(nowPath))
	err = ioutil.WriteFile(path, []byte(fmt.Sprintf("%s\n%s", c, b)), os.FileMode(0644))
	if err != nil {
		return err
	}

	cmd := exec.Command("vim", path, "-c", ":setlocal spell | silent %s/\r//g")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create new standup: %v", err)
	}

	l := &Local{standup: s.standup}
	b, err = l.readStandupFile(path)
	if err != nil {
		return err
	}

	regex, err := regexp.Compile("\n\n")
	if err != nil {
		return err
	}
	c = regex.ReplaceAllString(b, "\n")

	var out string
	first := true
	for _, str := range strings.Split(c, "\n") {
		if !strings.HasPrefix(str, "#") && len(str) > 0 && str[0] != '\n' {
			if first {
				out = str
				first = false
				continue
			}

			out = fmt.Sprintf("%s\n%s", out, str)
		}
	}

	if err := s.writeFile(out, nowPath); err != nil {
		return err
	}

	return nil
}

func (s *SSH) writeFile(data, path string) error {
	if err := s.setupSession(); err != nil {
		return err
	}

	if err := s.session.Run(fmt.Sprintf("echo '%s' > %s", data, path)); err != nil {
		return err
	}

	return nil
}

func (s *SSH) loadComment(path string) (string, error) {
	c, err := s.readStandupFile(path)
	if err != nil {
		return "", err
	}

	var out string
	for _, str := range strings.Split(c, "\n") {
		if len(str) > 0 && str[0] != '\n' {
			out = fmt.Sprintf("%s# %s\n", out, str)
		}
	}

	return out, nil
}

func (s *SSH) readStandupFile(path string) (string, error) {
	var buff bytes.Buffer
	writer := bufio.NewWriter(&buff)

	if err := s.setupSession(); err != nil {
		return "", err
	}

	s.session.Stdout = writer

	var c string
	if err := s.session.Run(fmt.Sprintf("cat %s", path)); err != nil {
		if err.Error() != "Process exited with status 1" {
			return "", fmt.Errorf("failed to cat file from ssh: %v", err)
		}
	} else {
		c = buff.String()
	}

	s.session.Close()

	return c, nil
}

func (s *SSH) readPubKey() error {
	path, err := homedir.Expand("~/.ssh/id_rsa")
	if err != nil {
		return err
	}

	buffer, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}

	s.config = &ssh.ClientConfig{
		User: s.standup.config.SshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	return nil
}

func (s *SSH) setupSession() error {
	connection, err := ssh.Dial("tcp", fmt.Sprintf("[%s]:22", s.standup.config.SshHost), s.config)
	if err != nil {
		return fmt.Errorf("failed to dial to ssh connection: %v", err)
	}

	session, err := connection.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create new session: %v", err)
	}

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stdout

	modes := ssh.TerminalModes{
		ssh.ECHO: 1,
	}

	fd := os.Stdin.Fd()

	var termWidth, termHeight int
	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return err
		}

		defer term.RestoreTerminal(fd, oldState)

		winsize, err := term.GetWinsize(fd)
		if err == nil {
			termWidth = int(winsize.Width)
			termHeight = int(winsize.Height)
		}
	}

	if err := session.RequestPty("xterm", termHeight, termWidth, modes); err != nil {
		return fmt.Errorf("failed to request pty: %v", err)
	}

	s.session = session

	return nil
}
