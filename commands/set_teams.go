package commands

import (
	"github.com/concourse/atc"
	"io/ioutil"
	"fmt"
	"gopkg.in/yaml.v2"
	"github.com/concourse/fly/rc"
)

type Teams struct {
	Teams []atc.Team
}

type SetTeamsCommand struct {
	Config        	string               `short:"c" long:"config" required:"true" description:"File to read team configuration from"`
}

func (command *SetTeamsCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	teamsConfig, err := ioutil.ReadFile(string(command.Config))
	if err != nil {
		return fmt.Errorf("could not read config file: %s", err.Error())
	}

	var teams Teams
	err = yaml.Unmarshal([]byte(teamsConfig), &teams)
	if err != nil {
		return err
	}

	var errors []error
	for _, team := range teams.Teams {
		_, created, updated, err := target.Client().Team(team.Name).CreateOrUpdate(team)
		if err != nil {
			errors = append(errors, err)
		}

		if created {
			fmt.Printf("team %s created", team.Name)
		} else if updated {
			fmt.Printf("team %s updated", team.Name)
		}
	}

	if len(errors) != 0 {
		fmt.Println("The following errors occurred:")
		for _, error := range errors {
			fmt.Printf("Error: %v", error)
		}
	}

	return nil
}
