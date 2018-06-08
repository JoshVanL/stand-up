package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/nlopes/slack"
	"github.com/spf13/cobra"
)

const (
	FlagStandUpDir = "stand-ups"
	FlagToken      = "token"
	FlagChannel    = "channel"
)

type StandUp struct {
	client  *slack.Client
	channel string
	token   string
	dir     string
}

var RootCmd = &cobra.Command{
	Use:   "stand-up",
	Short: "Post your stand-up to slack.",
	Run: func(cmd *cobra.Command, args []string) {

		day := time.Now().Weekday()

		if day == time.Saturday || day == time.Sunday {
			fmt.Printf("You're not working today!\n")
			os.Exit(0)
		}

		channel, err := cmd.PersistentFlags().GetString(FlagChannel)
		if err != nil {
			Must(fmt.Errorf("failed to get stand-up channel flag: %v", err))
		}

		path, err := cmd.PersistentFlags().GetString(FlagStandUpDir)
		if err != nil {
			Must(fmt.Errorf("failed to get stand-up directory flag: %v", err))
		}

		path, err = homedir.Expand(path)
		if err != nil {
			Must(fmt.Errorf("failed to expand stand-up directory: %v", err))
		}

		token, err := cmd.PersistentFlags().GetString(FlagToken)
		if err != nil {
			Must(fmt.Errorf("failed to get slack token flag: %v", err))
		}

		if token == "" {
			fmt.Printf("Slack token is not set, exiting.\n")
			os.Exit(1)
		}

		s := &StandUp{
			client:  slack.New(token),
			token:   token,
			dir:     path,
			channel: channel,
		}

		standup, err := s.CreateStandUp()
		Must(err)

		Must(s.SendStandUpMessage(standup))

	},
}

func main() {
	Must(RootCmd.Execute())
}

func init() {
	RootCmd.PersistentFlags().StringP(FlagStandUpDir, "f", "~/Jetstack/standups", "Set directory of standups")
	RootCmd.PersistentFlags().StringP(FlagToken, "t", "", "Set you client slack token")
	RootCmd.PersistentFlags().StringP(FlagChannel, "c", "stand-ups", "Set channel to post stand-up")
}

func Must(err error) {
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func (s *StandUp) SendStandUpMessage(standup string) error {
	_, _, err := s.client.PostMessage(s.channel, standup, slack.NewPostMessageParameters())
	return err
}

func (s *StandUp) CreateStandUp() (string, error) {
	now := time.Now()
	prevDay := now.Add(-time.Hour * 24)
	for prevDay.Weekday() == time.Saturday || prevDay.Weekday() == time.Sunday {
		prevDay = prevDay.Add(-time.Hour * 24)
	}

	todayPath := s.createPath(now)
	prevPath := s.createPath(prevDay)

	s1, err := ioutil.ReadFile(prevPath)
	if err != nil {
		return "", fmt.Errorf("failed to read last stand-up: %v", err)
	}

	s2, err := ioutil.ReadFile(todayPath)
	if err != nil {
		return "", fmt.Errorf("failed to read today's stand-up: %v", err)
	}

	standup := s.generateStandUp(s1, s2, now.Weekday().String(), prevDay.Weekday().String())

	return standup, nil
}

func (s *StandUp) generateStandUp(s1, s2 []byte, today, prevDay string) string {
	return fmt.Sprintf("```\n%s:\n%s\n\n%s:\n```", prevDay, s1, today, s2)
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
	path = filepath.Join(s.dir, path)

	return path
}
