package commands

import (
	"os"
	"sort"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type ContainersCommand struct{}

func (command *ContainersCommand) Execute([]string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}
	err = rc.ValidateClient(client, Fly.Target, false)
	if err != nil {
		return err
	}

	containers, err := client.ListContainers(map[string]string{})
	if err != nil {
		return err
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "handle", Color: color.New(color.Bold)},
			{Contents: "ttl", Color: color.New(color.Bold)},
			{Contents: "validity", Color: color.New(color.Bold)},
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
			{Contents: formatTTL(c.TTLInSeconds)},
			{Contents: formatTTL(c.ValidityInSeconds)},
			{Contents: c.WorkerName},
			stringOrDefault(c.PipelineName),
			stringOrDefault(c.JobName),
			stringOrDefault(c.BuildName),
			buildIDOrNone(c.BuildID),
			stringOrDefault(c.StepType, "check"),
			{Contents: (c.StepName + c.ResourceName)},
			stringOrDefault(SliceItoa(c.Attempts), "n/a"),
		}

		table.Data = append(table.Data, row)
	}

	sort.Sort(table.Data)

	return table.Render(os.Stdout)
}

type containersByHandle []atc.Container

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
