package main

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/robfig/cron"
	"golang.org/x/net/context"
	"strings"
	"time"
)

var WatchInterval = (5 * time.Second)

type ContainerStartJob struct {
	Client      *client.Client
	ContainerID string
	Context     context.Context
	Name        string
	Schedule    string
}

func (job ContainerStartJob) Run() {
	fmt.Println("Starting:", job.Name)
	err := job.Client.ContainerStart(job.Context, job.ContainerID, types.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}
}

func QueryScheduledJobs(cli *client.Client) (jobs []ContainerStartJob) {
	fmt.Println("Scanning containers for new schedules...")
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		if val, ok := container.Labels["cron.schedule"]; ok {
			jobName := strings.Join(container.Names, "/")
			jobs = append(jobs, ContainerStartJob{
				Schedule:    val,
				Client:      cli,
				ContainerID: container.ID,
				Context:     context.Background(),
				Name:        jobName,
			})
		}
	}

	return
}

func ScheduleJobs(c *cron.Cron, jobs []ContainerStartJob) {
	for _, job := range jobs {
		fmt.Printf("Scheduling %s (%s) with schedule '%s'\n", job.Name, job.ContainerID[:10], job.Schedule)
		c.AddJob(job.Schedule, job)
	}
}

func main() {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Create a cron
	c := cron.New()

	// Start the loop
	for {
		fmt.Println("Tick...")

		// HACK: This is risky as it could fall on the same interval as a task and that task would get skipped
		// It would be best to manage a ContainerID to Job mapping and then remove entries that are missing
		// in the new list and add new entries. However, cron does not support this yet.

		// Stop and create a new cron
		c.Stop()
		c = cron.New()

		// Schedule jobs again
		jobs := QueryScheduledJobs(cli)
		ScheduleJobs(c, jobs)
		c.Start()

		// Sleep until the next query time
		time.Sleep(WatchInterval)
	}
}
