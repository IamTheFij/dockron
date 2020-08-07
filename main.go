package main

import (
	"flag"
	"fmt"
	"log"
	"os"
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

	// version of dockron being run
	version = "dev"
)

// ContainerClient provides an interface for interracting with Docker
type ContainerClient interface {
	ContainerStart(context context.Context, containerID string, options dockerTypes.ContainerStartOptions) error
	ContainerList(context context.Context, options dockerTypes.ContainerListOptions) ([]dockerTypes.Container, error)
}

// ContainerStartJob represents a scheduled container task
// It contains a reference to a client, the schedule to run on, and the
// ID of that container that should be started
type ContainerStartJob struct {
	Client      ContainerClient
	ContainerID string
	Context     context.Context
	Name        string
	Schedule    string
}

// Run is executed based on the ContainerStartJob Schedule and starts the
// container
func (job ContainerStartJob) Run() {
	log.Println("Starting:", job.Name)
	err := job.Client.ContainerStart(job.Context, job.ContainerID, dockerTypes.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}
}

// QueryScheduledJobs queries Docker for all containers with a schedule and
// returns a list of ContainerStartJob records to be scheduled
func QueryScheduledJobs(client ContainerClient) (jobs []ContainerStartJob) {
	log.Println("Scanning containers for new schedules...")
	containers, err := client.ContainerList(context.Background(), dockerTypes.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		if val, ok := container.Labels[schedLabel]; ok {
			jobName := strings.Join(container.Names, "/")
			jobs = append(jobs, ContainerStartJob{
				Schedule:    val,
				Client:      client,
				ContainerID: container.ID,
				Context:     context.Background(),
				Name:        jobName,
			})
		}
	}

	return
}

// ScheduleJobs accepts a Cron instance and a list of jobs to schedule.
// It then schedules the provided jobs
func ScheduleJobs(c *cron.Cron, jobs []ContainerStartJob) {
	for _, job := range jobs {
		// TODO: Do something with the entryId returned here
		_, err := c.AddJob(job.Schedule, job)
		if err == nil {
			log.Printf("Scheduled %s (%s) with schedule '%s'\n", job.Name, job.ContainerID[:10], job.Schedule)
		} else {
			// TODO: Track something for a healthcheck here
			log.Printf("Error scheduling %s (%s) with schedule '%s'. %v\n", job.Name, job.ContainerID[:10], job.Schedule, err)
		}
	}
}

func main() {
	// Get a Docker Client
	client, err := dockerClient.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Read interval for polling Docker
	var watchInterval time.Duration
	flag.DurationVar(&watchInterval, "watch", defaultWatchInterval, "Interval used to poll Docker for changes")
	var showVersion = flag.Bool("version", false, "Display the version of dockron and exit")
	flag.Parse()

	// Print version if asked
	if *showVersion {
		fmt.Println("Dockron version:", version)
		os.Exit(0)
	}

	// Create a Cron
	c := cron.New()

	// Start the loop
	for {
		// HACK: This is risky as it could fall on the same interval as a task and that task would get skipped
		// It would be best to manage a ContainerID to Job mapping and then remove entries that are missing
		// in the new list and add new entries. However, cron does not support this yet.

		// Stop and create a new cron
		c.Stop()
		c = cron.New()

		jobs := QueryScheduledJobs(client)
		ScheduleJobs(c, jobs)

		c.Start()

		// Sleep until the next query time
		time.Sleep(watchInterval)
	}
}
