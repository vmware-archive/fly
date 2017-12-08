package commands

import "github.com/concourse/fly/rc"

type FlyCommand struct {
	Help HelpCommand `command:"help" description:"Print this help message"`

	Target  rc.TargetName  `short:"t" long:"target" description:"Concourse target name"`
	Targets TargetsCommand `command:"targets" alias:"ts" description:"List saved targets. https://concourse.ci/single-page.html#fly-targets"`

	Version func() `short:"v" long:"version" description:"Print the version of Fly and exit"`

	Verbose bool `long:"verbose" description:"Print API requests and responses"`

	PrintTableHeaders bool `long:"print-table-headers" description:"Print table headers even for redirected output"`

	Login  LoginCommand  `command:"login" alias:"l" description:"Authenticate with the target. https://concourse.ci/single-page.html#fly-login"`
	Logout LogoutCommand `command:"logout" alias:"o" description:"Release authentication with the target. https://concourse.ci/single-page.html#fly-logout"`
	Sync   SyncCommand   `command:"sync"  alias:"s" description:"Download and replace the current fly from the target. https://concourse.ci/single-page.html#fly-sync"`

	Teams       TeamsCommand       `command:"teams" alias:"t" description:"List the configured teams. https://concourse.ci/single-page.html#fly-teams"`
	SetTeam     SetTeamCommand     `command:"set-team"  alias:"st" description:"Create or modify a team to have the given credentials. https://concourse.ci/single-page.html#fly-set-team"`
	DestroyTeam DestroyTeamCommand `command:"destroy-team"  alias:"dt" description:"Destroy a team and delete all of its data. https://concourse.ci/single-page.html#fly-destroy-team"`

	Checklist ChecklistCommand `command:"checklist" alias:"cl" description:"Print a Checkfile of the given pipeline. https://concourse.ci/single-page.html#fly-checklist"`

	Execute ExecuteCommand `command:"execute" alias:"e" description:"Execute a one-off build using local bits. https://concourse.ci/single-page.html#fly-execute"`
	Watch   WatchCommand   `command:"watch"   alias:"w" description:"Stream a build's output. https://concourse.ci/single-page.html#fly-watch"`

	Containers ContainersCommand `command:"containers" alias:"cs" description:"Print the active containers. https://concourse.ci/single-page.html#fly-containers"`
	Hijack     HijackCommand     `command:"hijack"     alias:"intercept" alias:"i" description:"Execute a command in a container. https://concourse.ci/single-page.html#fly-intercept"`

	Jobs       JobsCommand       `command:"jobs"      alias:"js" description:"List the jobs in the pipelines. https://concourse.ci/single-page.html#fly-jobs"`
	PauseJob   PauseJobCommand   `command:"pause-job" alias:"pj" description:"Pause a job. https://concourse.ci/single-page.html#fly-pause-job"`
	UnpauseJob UnpauseJobCommand `command:"unpause-job" alias:"uj" description:"Unpause a job. https://concourse.ci/single-page.html#fly-unpause-job"`

	Pipelines        PipelinesCommand        `command:"pipelines"         alias:"ps" description:"List the configured pipelines. https://concourse.ci/single-page.html#fly-pipelines"`
	DestroyPipeline  DestroyPipelineCommand  `command:"destroy-pipeline"  alias:"dp" description:"Destroy a pipeline. https://concourse.ci/single-page.html#fly-destroy-pipeline"`
	GetPipeline      GetPipelineCommand      `command:"get-pipeline"      alias:"gp" description:"Get a pipeline's current configuration. https://concourse.ci/single-page.html#fly-get-pipeline"`
	SetPipeline      SetPipelineCommand      `command:"set-pipeline"      alias:"sp" description:"Create or update a pipeline's configuration. https://concourse.ci/single-page.html#fly-set-pipeline"`
	PausePipeline    PausePipelineCommand    `command:"pause-pipeline"    alias:"pp" description:"Pause a pipeline. https://concourse.ci/single-page.html#fly-pause-pipeline"`
	UnpausePipeline  UnpausePipelineCommand  `command:"unpause-pipeline"  alias:"up" description:"Un-pause a pipeline. https://concourse.ci/single-page.html#fly-unpause-pipeline"`
	ExposePipeline   ExposePipelineCommand   `command:"expose-pipeline"   alias:"ep" description:"Make a pipeline publicly viewable. https://concourse.ci/single-page.html#fly-expose-pipeline"`
	HidePipeline     HidePipelineCommand     `command:"hide-pipeline"     alias:"hp" description:"Hide a pipeline from the public. https://concourse.ci/single-page.html#fly-hide-pipeline"`
	RenamePipeline   RenamePipelineCommand   `command:"rename-pipeline"   alias:"rp" description:"Rename a pipeline. https://concourse.ci/single-page.html#fly-rename-pipeline"`
	ValidatePipeline ValidatePipelineCommand `command:"validate-pipeline" alias:"vp" description:"Validate a pipeline config. https://concourse.ci/single-page.html#fly-validate-pipeline"`
	FormatPipeline   FormatPipelineCommand   `command:"format-pipeline"   alias:"fp" description:"Format a pipeline config. https://concourse.ci/single-page.html#fly-format-pipeline"`

	CheckResource   CheckResourceCommand   `command:"check-resource"    alias:"cr" description:"Check a resource. https://concourse.ci/single-page.html#fly-check-resource"`
	PauseResource   PauseResourceCommand   `command:"pause-resource"    alias:"pr" description:"Pause a resource. https://concourse.ci/single-page.html#fly-pause-resource"`
	UnpauseResource UnpauseResourceCommand `command:"unpause-resource"  alias:"ur" description:"Unpause a resource. https://concourse.ci/single-page.html#fly-unpause-resource"`

	Builds     BuildsCommand     `command:"builds"      alias:"bs" description:"List builds data. https://concourse.ci/single-page.html#fly-builds"`
	AbortBuild AbortBuildCommand `command:"abort-build" alias:"ab" description:"Abort a build. https://concourse.ci/single-page.html#fly-abort-build"`

	TriggerJob TriggerJobCommand `command:"trigger-job" alias:"tj" description:"Start a job in a pipeline. https://concourse.ci/single-page.html#fly-trigger-job"`

	Volumes VolumesCommand `command:"volumes" alias:"vs" description:"List the active volumes. https://concourse.ci/single-page.html#fly-volumes"`

	Workers     WorkersCommand     `command:"workers" alias:"ws" description:"List the registered workers. https://concourse.ci/single-page.html#fly-workers"`
	PruneWorker PruneWorkerCommand `command:"prune-worker" alias:"pw" description:"Prune a stalled, landing, landed, or retiring worker. https://concourse.ci/single-page.html#fly-prune-worker"`
}

var Fly FlyCommand
