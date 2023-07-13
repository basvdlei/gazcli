// Pacakge cli contains the command line options and flags.
package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/basvdlei/gazcli/internal/azure"
	cli "github.com/urfave/cli/v2"
)

var appFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  "userid",
		Usage: "UserID (get it with: 'az ad signed-in-user show -o json | jq .id')",
	},
}
var appCommands = []*cli.Command{
	{
		Name:    "subscriptions",
		Aliases: []string{"s"},
		Usage:   "List enabled subscriptions",
		Action: func(c *cli.Context) error {
			s, err := azure.NewSession(c.Context, c.String("userid"))
			if err != nil {
				return err
			}
			subs, err := s.Subscriptions()
			if err != nil {
				return err
			}
			/*
				b, err := json.MarshalIndent(subs, "", "\t")
				if err != nil {
					return err
				}
				fmt.Printf("%s\n", b)
			*/
			for k := range subs {
				fmt.Println(k)
			}
			return nil
		},
	},
	{
		Name:      "roles",
		Aliases:   []string{"r"},
		Usage:     "List available roles",
		ArgsUsage: "<subscription>",
		Action: func(c *cli.Context) error {
			s, err := azure.NewSession(c.Context, c.String("userid"))
			if err != nil {
				return err
			}
			roles, err := s.RolesForSubscription(c.Args().Get(0))
			if err != nil {
				return err
			}
			/*
				b, err := json.MarshalIndent(subs, "", "\t")
				if err != nil {
					return err
				}
				fmt.Printf("%s\n", b)
			*/
			for _, v := range roles {
				fmt.Println(v)
			}
			return nil
		},
	},

	{
		Name:      "activate",
		Aliases:   []string{"a"},
		Usage:     "Active RoleAssignment for Subscription",
		ArgsUsage: "<subscription> <role> <justifiction> [duration]",
		Action: func(c *cli.Context) error {
			user := c.String("userid")
			if user == "" {
				return errors.New("userid flag not set")
			}
			args := c.Args()
			if args.Len() < 3 {
				return errors.New("not enough arguments")
			}
			s, err := azure.NewSession(c.Context, user)
			if err != nil {
				return err
			}
			subID := args.Get(0)
			roleDisplayName := args.Get(1)
			justifiction := args.Get(2)
			duration := 60 * time.Minute
			if args.Get(3) != "" {
				duration, err = time.ParseDuration(args.Get(3))
				if err != nil {
					return err
				}
			}
			return s.ActiveRoleAssignment(
				subID,
				roleDisplayName,
				justifiction,
				duration,
			)
		},
	},
}

func NewGazcli() error {
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Authors = []*cli.Author{
		{
			Name: "Bas van der Lei",
		},
	}
	app.Version = "0.0.1"
	app.Usage = "Go Azure CLI"
	app.Name = "gazcli"
	app.Flags = appFlags
	app.Commands = appCommands
	return app.Run(os.Args)
}
