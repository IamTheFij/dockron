package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"git.iamthefij.com/iamthefij/slog/v2"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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

// ContainerClient provides an interface for interracting with Docker. Makes it possible to mock in tests
type ContainerClient interface {
	ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (dockerTypes.IDResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error)
	ContainerExecStart(ctx context.Context, execID string, config container.ExecStartOptions) error
	ContainerExecAttach(ctx context.Context, execID string, options container.ExecAttachOptions) (dockerTypes.HijackedResponse, error)
	ContainerInspect(ctx context.Context, containerID string) (dockerTypes.ContainerJSON, error)
	ContainerList(context context.Context, options container.ListOptions) ([]dockerTypes.Container, error)
	ContainerStart(context context.Context, containerID string, options container.StartOptions) error
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
	slog.Infof("Starting: %s", job.name)

	// Check if container is already running
	containerJSON, err := job.client.ContainerInspect(
		job.context,
		job.containerID,
	)
	slog.OnErrPanicf(err, "Could not get container details for job %s", job.name)

	if containerJSON.State.Running {
		slog.Warningf("%s: Container is already running. Skipping start.", job.name)

		return
	}

	// Start job
	err = job.client.ContainerStart(
		job.context,
		job.containerID,
		container.StartOptions{},
	)
	slog.OnErrPanicf(err, "Could not start container for job %s", job.name)

	// Check results of job
	for check := true; check; check = containerJSON.State.Running {
		slog.Debugf("%s: Still running", job.name)

		containerJSON, err = job.client.ContainerInspect(
			job.context,
			job.containerID,
		)
		slog.OnErrPanicf(err, "Could not get container details for job %s", job.name)

		time.Sleep(1 * time.Second)
	}
	slog.Debugf("%s: Done running. %+v", job.name, containerJSON.State)

	// Log exit code if failed
	if containerJSON.State.ExitCode != 0 {
		slog.Errorf(
			"%s: Exec job exited with code %d",
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
	slog.Infof("Execing: %s", job.name)
	containerJSON, err := job.client.ContainerInspect(
		job.context,
		job.containerID,
	)
	slog.OnErrPanicf(err, "Could not get container details for job %s", job.name)

	if !containerJSON.State.Running {
		slog.Warningf("%s: Container not running. Skipping exec.", job.name)

		return
	}

	execID, err := job.client.ContainerExecCreate(
		job.context,
		job.containerID,
		container.ExecOptions{
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          []string{"sh", "-c", strings.TrimSpace(job.shellCommand)},
		},
	)
	slog.OnErrPanicf(err, "Could not create container exec job for %s", job.name)

	hj, err := job.client.ContainerExecAttach(job.context, execID.ID, container.ExecAttachOptions{})
	slog.OnErrWarnf(err, "%s: Error attaching to exec: %s", job.name, err)
	defer hj.Close()

	scanner := bufio.NewScanner(hj.Reader)

	err = job.client.ContainerExecStart(
		job.context,
		execID.ID,
		container.ExecStartOptions{},
	)
	slog.OnErrPanicf(err, "Could not start container exec job for %s", job.name)

	// Wait for job results
	execInfo := container.ExecInspect{Running: true}
	for execInfo.Running {
		time.Sleep(1 * time.Second)

		slog.Debugf("Still execing %s", job.name)
		execInfo, err = job.client.ContainerExecInspect(
			job.context,
			execID.ID,
		)

		// Maybe print output
		if hj.Reader != nil {
			for scanner.Scan() {
				line := scanner.Text()
				if len(line) > 0 {
					slog.Infof("%s: Exec output: %s", job.name, line)
				} else {
					slog.Debugf("%s: Empty exec output", job.name)
				}

				if err := scanner.Err(); err != nil {
					slog.OnErrWarnf(err, "%s: Error reading from exec", job.name)
				}
			}
		} else {
			slog.Debugf("%s: No exec reader", job.name)
		}

		slog.Debugf("%s: Exec info: %+v", job.name, execInfo)

		if err != nil {
			// Nothing we can do if we got an error here, so let's go
			slog.OnErrWarnf(err, "%s: Could not get status for exec job", job.name)

			return
		}
	}
	slog.Debugf("%s: Done execing. %+v", job.name, execInfo)
	// Log exit code if failed
	if execInfo.ExitCode != 0 {
		slog.Errorf("%s: Exec job existed with code %d", job.name, execInfo.ExitCode)
	}
}

// QueryScheduledJobs queries Docker for all containers with a schedule and
// returns a list of ContainerCronJob records to be scheduled
func QueryScheduledJobs(client ContainerClient) (jobs []ContainerCronJob) {
	slog.Debugf("Scanning containers for new schedules...")

	containers, err := client.ContainerList(
		context.Background(),
		container.ListOptions{All: true},
	)
	slog.OnErrPanicf(err, "Failure querying docker containers")

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
			expectedLabelParts := 3

			if len(results) == expectedLabelParts {
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

	return jobs
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
			slog.Debugf("Job %s is already scheduled. Skipping", job.Name())
			delete(existingJobs, job.UniqueName())

			continue
		}

		// Job doesn't exist yet, schedule it
		_, err := c.AddJob(job.Schedule(), job)
		if err == nil {
			slog.Infof(
				"Scheduled %s (%s) with schedule '%s'\n",
				job.Name(),
				job.UniqueName(),
				job.Schedule(),
			)
		} else {
			// TODO: Track something for a healthcheck here
			slog.Errorf(
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
	slog.OnErrPanicf(err, "Could not create Docker client")

	// Read interval for polling Docker
	var watchInterval time.Duration

	showVersion := flag.Bool("version", false, "Display the version of dockron and exit")

	flag.DurationVar(&watchInterval, "watch", defaultWatchInterval, "Interval used to poll Docker for changes")
	flag.BoolVar(&slog.DebugLevel, "debug", false, "Show debug logs")
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
