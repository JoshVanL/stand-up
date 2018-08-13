package main

import (
	"fmt"
	"os"
	"time"

	"github.com/nlopes/slack"
	"github.com/spf13/cobra"
)

const (
	FlagConfig = "config"
)

type StandUp struct {
	client   *slack.Client
	rtm      *slack.RTM
	id       string
	config   *Config
	provider Provider
}

type Provider interface {
	vimStandup(nowPath, yestPath string) error
	loadComment(path string) (string, error)
	readStandupFile(path string) (string, error)
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		Errorf("%s\n", err)
	}
}

func init() {
	RootCmd.PersistentFlags().StringP(FlagConfig, "c", "~/.config/stand-up.yaml", "Set stand-up config path")
	RootCmd.AddCommand(todayCmd)
	RootCmd.AddCommand(yesterdayCmd)
	RootCmd.AddCommand(showCmd)
}

var RootCmd = &cobra.Command{
	Use:   "stand-up",
	Short: "Post your stand-up to slack.",
	Run: func(cmd *cobra.Command, args []string) {

		s := NewStandup(cmd)

		s.client = slack.New(s.config.Token)

		standup, err := s.CreateStandUp()
		s.Must(err)

		rtm := s.client.NewRTM()
		rtm.GetChannels(true)
		users, err := rtm.GetUsers()
		if err != nil {
			Errorf("failed to get users: %v\n", err)
		}
		for _, u := range users {
			if u.Name == s.config.ClientName {
				s.id = u.ID
				break
			}
		}
		if s.id == "" {
			s.Must(fmt.Errorf("failed to find user: %s", s.config.ClientName))
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

	configPath, err := cmd.PersistentFlags().GetString(FlagConfig)
	if err != nil {
		Errorf("failed to parse config flag: %v", err)
	}

	config, err := NewConfig(configPath)
	if err != nil {
		Errorf("failed to parse config file: %v", err)
	}

	s.config = config
	if config.SshUser != "" && config.SshHost != "" {
		s.provider = &SSH{
			standup: s,
		}
	} else {
		s.provider = &Local{
			standup: s,
		}
	}

	return s
}

var todayCmd = &cobra.Command{
	Use:     "today",
	Aliases: []string{"tod", "t", "to"},
	Short:   "Open vim for today's stand-up",
	Run: func(cmd *cobra.Command, args []string) {
		s := NewStandup(RootCmd)
		s.Must(s.provider.vimStandup(s.createPath(time.Now()), s.createPath(s.prevDay())))
	},
}

var yesterdayCmd = &cobra.Command{
	Use:     "yesterday",
	Aliases: []string{"yes", "y"},
	Short:   "Open vim for yesterday's stand-up",
	Run: func(cmd *cobra.Command, args []string) {
		s := NewStandup(RootCmd)
		s.Must(s.provider.vimStandup(s.createPath(s.prevDay()), s.createPath(s.prevPrevDay())))
	},
}

var showCmd = &cobra.Command{
	Use:     "show",
	Aliases: []string{"s"},
	Short:   "Print today's stand-up",
	Run: func(cmd *cobra.Command, args []string) {
		s := NewStandup(RootCmd)

		b, err := s.CreateStandUp()
		if err != nil {
			s.Must(err)
		}

		fmt.Printf("%s", b)
	},
}

func (s *StandUp) SendStandUpMessage(standup string) error {
	params := slack.NewPostMessageParameters()
	params.AsUser = true
	respChanel, respTimestamp, err := s.client.PostMessage(s.config.Channel, standup, params)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n%s\n", respChanel, respTimestamp)
	return nil
}

func Error(err string) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}

func Errorf(err string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, err, a)
	os.Exit(1)
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
