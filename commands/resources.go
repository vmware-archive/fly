package commands

import (
	"os"
	"sort"
	"strconv"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type ResourcesCommand struct {
	Json     bool `long:"json"  description:"Print command result as JSON"`
	Detailed bool `short:"d"  long:"detailed"  description:"Get detailed information such as resource name and pipeline name"`
}

func (command *ResourcesCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	detailed := command.Detailed

	containers, err := target.Team().ListCheckContainerDetails(detailed)
	if err != nil {
		return err
	}

	if command.Json {
		err = displayhelpers.JsonPrint(containers)
		if err != nil {
			return err
		}
		return nil
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "handle", Color: color.New(color.Bold)},
			{Contents: "worker", Color: color.New(color.Bold)},
			{Contents: "container-type", Color: color.New(color.Bold)},
			{Contents: "resource-config-id", Color: color.New(color.Bold)},
			{Contents: "resource-type", Color: color.New(color.Bold)},
		},
	}

	if detailed {
		table.Headers = append(table.Headers,
			ui.TableCell{Contents: "pipeline", Color: color.New(color.Bold)},
			ui.TableCell{Contents: "resource-name", Color: color.New(color.Bold)},
		)
	}

	for _, c := range containers {
		row := ui.TableRow{
			{Contents: c.ID},
			{Contents: c.WorkerName},
			{Contents: c.Type},
			intOrDefault(c.ResourceConfigID),
			stringOrDefault(c.ResourceTypeName),
		}

		if detailed {
			row = append(row,
				stringOrDefault(c.PipelineName),
				stringOrDefault(c.ResourceName),
			)
		}

		table.Data = append(table.Data, row)
	}

	sort.Sort(table.Data)

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func intOrDefault(id int) ui.TableCell {
	var column ui.TableCell

	if id == 0 {
		column.Contents = "none"
		column.Color = color.New(color.Faint)
	} else {
		column.Contents = strconv.Itoa(id)
	}

	return column
}
