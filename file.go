package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v2"
)

type Config struct {
	SshUser    string `yaml:"sshUser"`
	SshHost    string `yaml:"sshHost"`
	Dir        string `yaml:"dir"`
	Token      string `yaml:"token"`
	ClientName string `yaml:"clientName"`
	Channel    string `yaml:"channel"`
}

func NewConfig(path string) (*Config, error) {
	expand, err := homedir.Expand(path)
	if err != nil {
		return nil, err
	}

	var b []byte
	if _, err := os.Stat(expand); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		b, err = ioutil.ReadFile(expand)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %v", err)
		}
	}

	config := new(Config)
	if err := yaml.Unmarshal(b, config); err != nil {
		return nil, err
	}

	var result *multierror.Error
	for _, s := range []struct {
		name, content string
	}{
		{name: "dir", content: config.Dir},
		{name: "token", content: config.Token},
		{name: "clientName", content: config.ClientName},
		{name: "channel", content: config.Channel},
	} {
		if s.content == "" {
			result = multierror.Append(result, fmt.Errorf("%s is empty", s.name))
		}
	}

	return config, result.ErrorOrNil()
}

func (s *StandUp) CreateStandUp() (string, error) {
	now := time.Now()
	prevDay := s.prevDay()

	todayPath := s.createPath(now)
	prevPath := s.createPath(prevDay)

	s1, err := s.provider.readStandupFile(prevPath)
	if err != nil {
		return "", fmt.Errorf("failed to read last stand-up: %v", err)
	}

	s2, err := s.provider.readStandupFile(todayPath)
	if err != nil {
		return "", fmt.Errorf("failed to read today's stand-up: %v", err)
	}

	standup := s.generateStandUp(s1, s2, now.Weekday().String(), prevDay.Weekday().String())

	return standup, nil
}

func (s *StandUp) prevDay() time.Time {
	prevDay := time.Now().Add(-time.Hour * 24)
	for prevDay.Weekday() == time.Saturday || prevDay.Weekday() == time.Sunday {
		prevDay = prevDay.Add(-time.Hour * 24)
	}

	return prevDay
}

func (s *StandUp) createPath(t time.Time) string {
	day := strconv.Itoa(t.Day())
	if len(day) < 2 {
		day = fmt.Sprintf("0%s", day)
	}

	month := strconv.Itoa(int(t.Month()))
	if len(month) < 2 {
		month = fmt.Sprintf("0%s", month)
	}

	path := fmt.Sprintf("%s_%s_%d", day, month, (t.Year() % 100))
	path = filepath.Join(s.config.Dir, path)

	return path
}

func (s *StandUp) generateStandUp(s1, s2, today, prevDay string) string {
	return fmt.Sprintf("```\n%s:\n%s\n\n%s:\n%s\n```", prevDay, s1, today, s2)
}

func (s *StandUp) prevPrevDay() time.Time {
	prevPrevDay := time.Now().Add(-time.Hour * 48)
	for prevPrevDay.Weekday() == time.Saturday || prevPrevDay.Weekday() == time.Sunday {
		prevPrevDay = prevPrevDay.Add(-time.Hour * 24)
	}

	return prevPrevDay
}
