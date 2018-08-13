package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type Local struct {
	standup *StandUp
}

func (l *Local) vimStandup(nowPath, yestPath string) error {
	cmd := exec.Command("vim", nowPath, "-c", ":setlocal spell")

	c, err := l.loadComment(yestPath)
	if err != nil {
		return err
	}

	b, err := l.readStandupFile(nowPath)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(nowPath, []byte(fmt.Sprintf("%s\n%s", c, b)), os.FileMode(0644))
	if err != nil {
		return err
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create new standup: %v", err)
	}

	b, err = l.readStandupFile(nowPath)
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

	return ioutil.WriteFile(nowPath, []byte(out), os.FileMode(0644))
}

func (l *Local) readStandupFile(path string) (string, error) {
	var b []byte
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
	} else {
		b, err = ioutil.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read stand-up file: %v", err)
		}
	}

	return string(b), nil
}

func (l *Local) loadComment(path string) (string, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to get previous stand-up: %v", err)
	}

	var out string
	for _, str := range strings.Split(string(b), "\n") {
		out = fmt.Sprintf("%s# %s\n", out, str)
	}

	return out, nil
}
