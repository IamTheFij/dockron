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

type ContainerStartJob struct {
	Client      *client.Client
	ContainerID string
	Context     context.Context
	Name        string
	Schedule    string
}

func (job ContainerStartJob) Run() {
	fmt.Println("Starting: ", job.Name)
	err := job.Client.ContainerStart(job.Context, job.ContainerID, types.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}
}

func main() {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	jobs := []ContainerStartJob{}

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

	c := cron.New()

	for _, job := range jobs {
		fmt.Println("Scheduling ", job.Name, "(", job.Schedule, ")")
		c.AddJob(job.Schedule, job)
	}

	// Start the cron job threads
	c.Start()

	// Start the loop
	for {
		time.Sleep(5 * time.Second)
		fmt.Println("Tick...")
	}
}
