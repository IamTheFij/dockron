package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	dockerClient "github.com/docker/docker/client"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/context"
)

var (
	// defaultWatchInterval is the duration we should sleep until polling Docker
	defaultWatchInterval = (1 * time.Minute)

	// schedLabel is the string label to search for cron expressions
	schedLabel = "dockron.schedule"
	// execLabelRegex is will capture labels for an exec job
	execLabelRegexp = regexp.MustCompile(`dockron\.([a-zA-Z0-9_-]+)\.(schedule|command)`)

	// version of dockron being run
	version = "dev"
)

// ContainerClient provides an interface for interracting with Docker
type ContainerClient interface {
	ContainerExecCreate(ctx context.Context, container string, config dockerTypes.ExecConfig) (dockerTypes.IDResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (dockerTypes.ContainerExecInspect, error)
	ContainerExecStart(ctx context.Context, execID string, config dockerTypes.ExecStartCheck) error
	ContainerInspect(ctx context.Context, containerID string) (dockerTypes.ContainerJSON, error)
	ContainerList(context context.Context, options dockerTypes.ContainerListOptions) ([]dockerTypes.Container, error)
	ContainerStart(context context.Context, containerID string, options dockerTypes.ContainerStartOptions) error
}

// ContainerCronJob is an interface of a job to run on containers
type ContainerCronJob interface {
	Run()
	Name() string
	UniqueName() string
	Schedule() string
}

// ContainerStartJob represents a scheduled container task
// It contains a reference to a client, the schedule to run on, and the
// ID of that container that should be started
type ContainerStartJob struct {
	client      ContainerClient
	context     context.Context
	name        string
	containerID string
	schedule    string
}

// Run is executed based on the ContainerStartJob Schedule and starts the
// container
func (job ContainerStartJob) Run() {
	log.Println("Starting:", job.name)

	// Check if container is already running
	containerJSON, err := job.client.ContainerInspect(
		job.context,
		job.containerID,
	)
	PanicErr(err, "Could not get container details for job %s", job.name)

	if containerJSON.State.Running {
		LogWarning("Container is already running. Skipping %s", job.name)
		return
	}

	// Start job
	err = job.client.ContainerStart(
		job.context,
		job.containerID,
		dockerTypes.ContainerStartOptions{},
	)
	PanicErr(err, "Could not start container for jobb %s", job.name)

	// Check results of job
	for check := true; check; check = containerJSON.State.Running {
		LogDebug("Still running %s", job.name)

		containerJSON, err = job.client.ContainerInspect(
			job.context,
			job.containerID,
		)
		PanicErr(err, "Could not get container details for job %s", job.name)

		time.Sleep(1 * time.Second)
	}
	LogDebug("Done execing %s. %+v", job.name, containerJSON.State)
	// Log exit code if failed
	if containerJSON.State.ExitCode != 0 {
		LogError(
			"Exec job %s existed with code %d",
			job.name,
			containerJSON.State.ExitCode,
		)
	}

}

// Name returns the name of the job
func (job ContainerStartJob) Name() string {
	return job.name
}

// Schedule returns the schedule of the job
func (job ContainerStartJob) Schedule() string {
	return job.schedule
}

// UniqueName returns a unique identifier for a container start job
func (job ContainerStartJob) UniqueName() string {
	// ContainerID should be unique as a change in label will result in
	// a new container as they are immutable
	return job.name + "/" + job.containerID
}

// ContainerExecJob is a scheduled job to be executed in a running container
type ContainerExecJob struct {
	ContainerStartJob
	shellCommand string
}

// Run is executed based on the ContainerStartJob Schedule and starts the
// container
func (job ContainerExecJob) Run() {
	log.Println("Execing:", job.name)
	containerJSON, err := job.client.ContainerInspect(
		job.context,
		job.containerID,
	)
	PanicErr(err, "Could not get container details for job %s", job.name)

	if !containerJSON.State.Running {
		LogWarning("Container not running. Skipping %s", job.name)
		return
	}

	execID, err := job.client.ContainerExecCreate(
		job.context,
		job.containerID,
		dockerTypes.ExecConfig{
			Cmd: []string{"sh", "-c", strings.TrimSpace(job.shellCommand)},
		},
	)
	PanicErr(err, "Could not create container exec job for %s", job.name)

	err = job.client.ContainerExecStart(
		job.context,
		execID.ID,
		dockerTypes.ExecStartCheck{},
	)
	PanicErr(err, "Could not start container exec job for %s", job.name)

	// Wait for job results
	execInfo := dockerTypes.ContainerExecInspect{Running: true}
	for execInfo.Running {
		LogDebug("Still execing %s", job.name)
		execInfo, err = job.client.ContainerExecInspect(
			job.context,
			execID.ID,
		)
		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}
	LogDebug("Done execing %s. %+v", job.name, execInfo)
	// Log exit code if failed
	if execInfo.ExitCode != 0 {
		LogError("Exec job %s existed with code %d", job.name, execInfo.ExitCode)
	}
}

// QueryScheduledJobs queries Docker for all containers with a schedule and
// returns a list of ContainerCronJob records to be scheduled
func QueryScheduledJobs(client ContainerClient) (jobs []ContainerCronJob) {
	LogDebug("Scanning containers for new schedules...")

	containers, err := client.ContainerList(
		context.Background(),
		dockerTypes.ContainerListOptions{All: true},
	)
	PanicErr(err, "Failure querying docker containers")

	for _, container := range containers {
		// Add start job
		if val, ok := container.Labels[schedLabel]; ok {
			jobName := strings.Join(container.Names, "/")
			jobs = append(jobs, ContainerStartJob{
				client:      client,
				containerID: container.ID,
				context:     context.Background(),
				schedule:    val,
				name:        jobName,
			})
		}

		// Add exec jobs
		execJobs := map[string]map[string]string{}
		for label, value := range container.Labels {
			results := execLabelRegexp.FindStringSubmatch(label)
			if len(results) == 3 {
				// We've got part of a new job
				jobName, jobField := results[1], results[2]
				if partJob, ok := execJobs[jobName]; ok {
					// Partial exists, add the other value
					partJob[jobField] = value
				} else {
					// No partial exists, add this part
					execJobs[jobName] = map[string]string{
						jobField: value,
					}
				}
			}
		}
		for jobName, jobConfig := range execJobs {
			schedule, ok := jobConfig["schedule"]
			if !ok {
				continue
			}
			shellCommand, ok := jobConfig["command"]
			if !ok {
				continue
			}
			jobs = append(jobs, ContainerExecJob{
				ContainerStartJob: ContainerStartJob{
					client:      client,
					containerID: container.ID,
					context:     context.Background(),
					schedule:    schedule,
					name:        strings.Join(append(container.Names, jobName), "/"),
				},
				shellCommand: shellCommand,
			})
		}
	}

	return
}

// ScheduleJobs accepts a Cron instance and a list of jobs to schedule.
// It then schedules the provided jobs
func ScheduleJobs(c *cron.Cron, jobs []ContainerCronJob) {
	// Fetch existing jobs from the cron
	existingJobs := map[string]cron.EntryID{}
	for _, entry := range c.Entries() {
		// This should be safe since ContainerCronJob is the only type of job we use
		existingJobs[entry.Job.(ContainerCronJob).UniqueName()] = entry.ID
	}

	for _, job := range jobs {
		if _, ok := existingJobs[job.UniqueName()]; ok {
			// Job already exists, remove it from existing jobs so we don't
			// unschedule it later
			LogDebug("Job %s is already scheduled. Skipping", job.Name())
			delete(existingJobs, job.UniqueName())
			continue
		}

		// Job doesn't exist yet, schedule it
		_, err := c.AddJob(job.Schedule(), job)
		if err == nil {
			log.Printf(
				"Scheduled %s (%s) with schedule '%s'\n",
				job.Name(),
				job.UniqueName(),
				job.Schedule(),
			)
		} else {
			// TODO: Track something for a healthcheck here
			LogError(
				"Could not schedule %s (%s) with schedule '%s'. %v\n",
				job.Name(),
				job.UniqueName(),
				job.Schedule(),
				err,
			)
		}
	}

	// Remove remaining scheduled jobs that weren't in the new list
	for _, entryID := range existingJobs {
		c.Remove(entryID)
	}
}

func main() {
	// Get a Docker Client
	client, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv)
	if err != nil {
		panic(err)
	}

	// Read interval for polling Docker
	var watchInterval time.Duration
	flag.DurationVar(&watchInterval, "watch", defaultWatchInterval, "Interval used to poll Docker for changes")
	var showVersion = flag.Bool("version", false, "Display the version of dockron and exit")
	flag.BoolVar(&DebugLevel, "debug", false, "Show debug logs")
	flag.Parse()

	// Print version if asked
	if *showVersion {
		fmt.Println("Dockron version:", version)
		os.Exit(0)
	}

	// Create a Cron
	c := cron.New()
	c.Start()

	// Start the loop
	for {
		// Schedule jobs again
		jobs := QueryScheduledJobs(client)
		ScheduleJobs(c, jobs)

		// Sleep until the next query time
		time.Sleep(watchInterval)
	}
}
