package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/nlopes/slack"
	"github.com/spf13/cobra"
)

const (
	FlagStandUpDir = "stand-ups"
	FlagToken      = "token"
	FlagChannel    = "channel"
	FlagName       = "name"
)

type StandUp struct {
	client *slack.Client
	rtm    *slack.RTM
	id     string

	channel string
	name    string
	token   string
	dir     string
}

var RootCmd = &cobra.Command{
	Use:   "stand-up",
	Short: "Post your stand-up to slack.",
	Run: func(cmd *cobra.Command, args []string) {

		s := NewStandup(cmd)

		token, err := cmd.PersistentFlags().GetString(FlagToken)
		if err != nil {
			Errorf("failed to get slack token flag: %v", err)
		}
		if token == "" {
			Error("Slack token not set, exiting.\n")
		}

		s.token = token
		s.client = slack.New(token)

		name, err := cmd.PersistentFlags().GetString(FlagName)
		if err != nil {
			Errorf("failed to get client name flag: %v", err)
		}
		s.name = name

		channel, err := cmd.PersistentFlags().GetString(FlagChannel)
		if err != nil {
			Errorf("failed to get stand-up channel flag: %v", err)
		}
		s.channel = channel

		standup, err := s.CreateStandUp()
		s.Must(err)

		rtm := s.client.NewRTM()
		rtm.GetChannels(true)
		users, err := rtm.GetUsers()
		if err != nil {
			Errorf("failed to get users: %v", err)
		}
		for _, u := range users {
			if u.Name == s.name {
				s.id = u.ID
				break
			}
		}
		if s.id == "" {
			s.Must(fmt.Errorf("failed to find user: %s", s.name))
		}
		s.rtm = rtm

		s.Must(s.SendStandUpMessage(standup))
	},
}

func NewStandup(cmd *cobra.Command) *StandUp {
	day := time.Now().Weekday()

	if day == time.Saturday || day == time.Sunday {
		Error("You're not working today!\n")
	}

	s := new(StandUp)

	path, err := cmd.PersistentFlags().GetString(FlagStandUpDir)
	if err != nil {
		s.Must(fmt.Errorf("failed to get stand-up directory flag: %v", err))
	}
	path, err = homedir.Expand(path)
	if err != nil {
		s.Must(fmt.Errorf("failed to expand stand-up directory: %v", err))
	}
	s.dir = path

	return s
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		Errorf("%v\n", err)
	}
}

var todayCmd = &cobra.Command{
	Use:     "today",
	Aliases: []string{"tod", "t", "to"},
	Short:   "Open vim for today's stand-up",
	Run: func(cmd *cobra.Command, args []string) {
		s := NewStandup(RootCmd)
		s.Must(s.vimStandup(s.createPath(time.Now()), s.createPath(s.prevDay())))
	},
}

var yesterdayCmd = &cobra.Command{
	Use:     "yesterday",
	Aliases: []string{"yes", "y"},
	Short:   "Open vim for yesterday's stand-up",
	Run: func(cmd *cobra.Command, args []string) {
		s := NewStandup(RootCmd)
		s.Must(s.vimStandup(s.createPath(s.prevDay()), s.createPath(s.prevPrevDay())))
	},
}

func init() {
	RootCmd.PersistentFlags().StringP(FlagStandUpDir, "f", "/home/josh/Jetstack/standups", "Set directory of standups")
	RootCmd.PersistentFlags().StringP(FlagToken, "t", "", "Set you client slack token")
	RootCmd.PersistentFlags().StringP(FlagChannel, "c", "stand-ups", "Set channel to post stand-up")
	RootCmd.PersistentFlags().StringP(FlagName, "n", "joshua.vanleeuwen", "Set name of slack client")
	RootCmd.AddCommand(todayCmd)
	RootCmd.AddCommand(yesterdayCmd)
}

func (s *StandUp) Must(err error) {
	if err != nil {
		if s.rtm != nil {
			params := slack.NewPostMessageParameters()
			params.AsUser = false
			errStr := fmt.Sprintf("An error occured when trying to post your stand up!\n%v\n", err)
			_, _, err := s.rtm.PostMessage(s.id, errStr, params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "an error occured sending error to slack client: %v\n", err)
			}
		}

		Errorf("%v\n", err)
	}
}

func Error(err string) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}

func Errorf(err string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, err, a)
	os.Exit(1)
}

func (s *StandUp) SendStandUpMessage(standup string) error {
	params := slack.NewPostMessageParameters()
	params.AsUser = true
	respChanel, respTimestamp, err := s.client.PostMessage(s.channel, standup, params)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n%s\n", respChanel, respTimestamp)
	return nil
}

func (s *StandUp) CreateStandUp() (string, error) {
	now := time.Now()
	prevDay := s.prevDay()

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

func (s *StandUp) prevDay() time.Time {
	prevDay := time.Now().Add(-time.Hour * 24)
	for prevDay.Weekday() == time.Saturday || prevDay.Weekday() == time.Sunday {
		prevDay = prevDay.Add(-time.Hour * 24)
	}

	return prevDay
}

func (s *StandUp) prevPrevDay() time.Time {
	prevPrevDay := time.Now().Add(-time.Hour * 48)
	for prevPrevDay.Weekday() == time.Saturday || prevPrevDay.Weekday() == time.Sunday {
		prevPrevDay = prevPrevDay.Add(-time.Hour * 24)
	}

	return prevPrevDay
}

func (s *StandUp) generateStandUp(s1, s2 []byte, today, prevDay string) string {
	return fmt.Sprintf("```\n%s:\n%s\n%s:\n%s```", prevDay, s1, today, s2)
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

func (s *StandUp) vimStandup(nowPath, yestPath string) error {
	cmd := exec.Command("vim", nowPath, "-c", ":setlocal spell")

	c, err := s.loadComment(yestPath)
	if err != nil {
		return err
	}

	var b []byte
	if _, err := os.Stat(nowPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		b, err = ioutil.ReadFile(nowPath)
		if err != nil {
			return fmt.Errorf("failed to read todays stand-up: %v", err)
		}
	}

	err = ioutil.WriteFile(nowPath, []byte(fmt.Sprintf("%s\n%s", c, string(b))), os.FileMode(0644))
	if err != nil {
		return err
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create new standup: %v", err)
	}

	b, err = ioutil.ReadFile(nowPath)
	if err != nil {
		return err
	}

	regex, err := regexp.Compile("\n\n")
	if err != nil {
		return err
	}
	c = regex.ReplaceAllString(string(b), "\n")

	var out string
	for _, str := range strings.Split(c, "\n") {
		if !strings.HasPrefix(str, "#") {
			out = fmt.Sprintf("%s%s", out, str)
		}
	}

	return ioutil.WriteFile(nowPath, []byte(out), os.FileMode(0644))
}

func (s *StandUp) loadComment(path string) (string, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to get previous stand-up: %v", err)
	}

	n := strings.Count(string(b), "\n") - 1
	if n < 0 {
		n = 0
	}

	return fmt.Sprintf("# %s", strings.Replace(string(b), "\n", "\n# ", n)), nil
}
