package commands

import (
	"sort"
	"strconv"

	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type ContainersCommand struct{}

func (command *ContainersCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	containers, err := target.Client().ListContainers(map[string]string{})
	if err != nil {
		return err
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "handle", Color: color.New(color.Bold)},
			{Contents: "worker", Color: color.New(color.Bold)},
			{Contents: "pipeline", Color: color.New(color.Bold)},
			{Contents: "job", Color: color.New(color.Bold)},
			{Contents: "build #", Color: color.New(color.Bold)},
			{Contents: "build id", Color: color.New(color.Bold)},
			{Contents: "type", Color: color.New(color.Bold)},
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "attempt", Color: color.New(color.Bold)},
		},
	}

	for _, c := range containers {
		row := ui.TableRow{
			{Contents: c.ID},
			{Contents: c.WorkerName},
			stringOrDefault(c.PipelineName),
			stringOrDefault(c.JobName),
			stringOrDefault(c.BuildName),
			buildIDOrNone(c.BuildID),
			{Contents: c.Type},
			stringOrDefault(c.StepName + c.ResourceName),
			stringOrDefault(c.Attempt, "n/a"),
		}

		table.Data = append(table.Data, row)
	}

	sort.Sort(table.Data)

	return table.Render(color.Output, Fly.PrintTableHeaders)
}

func buildIDOrNone(id int) ui.TableCell {
	var column ui.TableCell

	if id == 0 {
		column.Contents = "none"
		column.Color = color.New(color.Faint)
	} else {
		column.Contents = strconv.Itoa(id)
	}

	return column
}

func stringOrDefault(containerType string, def ...string) ui.TableCell {
	var column ui.TableCell

	column.Contents = containerType
	if column.Contents == "" || column.Contents == "[]" {
		if len(def) == 0 {
			column.Contents = "none"
			column.Color = color.New(color.Faint)
		} else {
			column.Contents = def[0]
		}
	}

	return column
}
