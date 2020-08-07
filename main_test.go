package main

import (
	"fmt"
	"log"
	"testing"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/context"
)

// FakeDockerClient is used to test without interracting with Docker
type FakeDockerClient struct {
	FakeContainers []dockerTypes.Container
}

// ContainerStart pretends to start a container
func (fakeClient *FakeDockerClient) ContainerStart(context context.Context, containerID string, options dockerTypes.ContainerStartOptions) error {
	return nil
}

func (fakeClient *FakeDockerClient) ContainerList(context context.Context, options dockerTypes.ContainerListOptions) ([]dockerTypes.Container, error) {
	return fakeClient.FakeContainers, nil
}

// NewFakeDockerClient creates an empty client
func NewFakeDockerClient() *FakeDockerClient {
	return &FakeDockerClient{}
}

// NewFakeDockerClientWithContainers creates a client with the provided containers
func NewFakeDockerClientWithContainers(containers []dockerTypes.Container) *FakeDockerClient {
	return &FakeDockerClient{FakeContainers: containers}
}

// ErrorUnequal checks that two values are equal and fails the test if not
func ErrorUnequal(t *testing.T, expected interface{}, actual interface{}, message string) {
	if expected != actual {
		t.Errorf("%s Expected: %+v Actual: %+v", message, expected, actual)
	}
}

// TestQueryScheduledJobs checks that when querying the Docker client that we
// create jobs for any containers with a dockron.schedule
func TestQueryScheduledJobs(t *testing.T) {
	client := NewFakeDockerClient()

	cases := []struct {
		name           string
		fakeContainers []dockerTypes.Container
		expectedJobs   []ContainerStartJob
	}{
		{
			name:           "No containers",
			fakeContainers: []dockerTypes.Container{},
			expectedJobs:   []ContainerStartJob{},
		},
		{
			name: "One container without schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"no_schedule_1"},
					ID:    "no_schedule_1",
				},
			},
			expectedJobs: []ContainerStartJob{},
		},
		{
			name: "One container with schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"has_schedule_1"},
					ID:    "has_schedule_1",
					Labels: map[string]string{
						"dockron.schedule": "* * * * *",
					},
				},
			},
			expectedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1",
					Schedule:    "* * * * *",
					Context:     context.Background(),
					Client:      client,
				},
			},
		},
		{
			name: "One container with and one without schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"no_schedule_1"},
					ID:    "no_schedule_1",
				},
				dockerTypes.Container{
					Names: []string{"has_schedule_1"},
					ID:    "has_schedule_1",
					Labels: map[string]string{
						"dockron.schedule": "* * * * *",
					},
				},
			},
			expectedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1",
					Schedule:    "* * * * *",
					Context:     context.Background(),
					Client:      client,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			log.Printf("Running %s", t.Name())

			// Load fake containers
			t.Logf("Fake containers: %+v", c.fakeContainers)
			client.FakeContainers = c.fakeContainers

			jobs := QueryScheduledJobs(client)

			t.Logf("Expected jobs: %+v, Actual jobs: %+v", c.expectedJobs, jobs)

			ErrorUnequal(t, len(c.expectedJobs), len(jobs), "Job lengths don't match")
			for i, job := range jobs {
				ErrorUnequal(t, c.expectedJobs[i], job, "Job value does not match")
			}
		})
	}
}

func TestScheduleJobs(t *testing.T) {
	c := cron.New()

	t.Run("Schedule nothing", func(t *testing.T) {
		log.Printf("Running %s", t.Name())
		jobs := []ContainerStartJob{}
		ScheduleJobs(c, jobs)

		scheduledEntries := c.Entries()

		ErrorUnequal(t, len(jobs), len(scheduledEntries), "Job lengths don't match")
		for i, job := range jobs {
			// Set client for comparison
			ErrorUnequal(t, job, scheduledEntries[i].Job, "Job value does not match")
		}
	})

	t.Run("Schedule a job", func(t *testing.T) {
		log.Printf("Running %s", t.Name())
		jobs := []ContainerStartJob{
			ContainerStartJob{
				ContainerID: "0123456789/has_schedule_1",
				Name:        "has_schedule_1",
				Schedule:    "* * * * *",
			},
		}
		ScheduleJobs(c, jobs)

		scheduledEntries := c.Entries()

		ErrorUnequal(t, len(jobs), len(scheduledEntries), "Job lengths don't match")
		for i, job := range jobs {
			// Set client for comparison
			ErrorUnequal(t, job, scheduledEntries[i].Job, "Job value does not match")
		}
	})

	// Subsequently scheduled jobs will append since we currently just stop and create a new cron
	// Eventually this test case should change when proper removal is supported

	t.Run("Schedule a second job", func(t *testing.T) {
		log.Printf("Running %s", t.Name())
		jobs := []ContainerStartJob{
			ContainerStartJob{
				ContainerID: "0123456789/has_schedule_2",
				Name:        "has_schedule_2",
				Schedule:    "* * * * *",
			},
		}
		ScheduleJobs(c, jobs)
		jobs = append([]ContainerStartJob{
			ContainerStartJob{
				ContainerID: "0123456789/has_schedule_1",
				Name:        "has_schedule_1",
				Schedule:    "* * * * *",
			},
		}, jobs...)

		scheduledEntries := c.Entries()

		ErrorUnequal(t, len(jobs), len(scheduledEntries), "Additional job didn't show")
		for i, job := range jobs {
			// Set client for comparison
			ErrorUnequal(t, job, scheduledEntries[i].Job, "Job value does not match")
		}
	})
}

func TestDoLoop(t *testing.T) {
	croner := cron.New()
	client := NewFakeDockerClient()

	cases := []struct {
		name           string
		fakeContainers []dockerTypes.Container
		expectedJobs   []ContainerStartJob
	}{
		{
			name:           "No containers",
			fakeContainers: []dockerTypes.Container{},
			expectedJobs:   []ContainerStartJob{},
		},
		{
			name: "One container without schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"no_schedule_1"},
					ID:    "no_schedule_1",
				},
			},
			expectedJobs: []ContainerStartJob{},
		},
		{
			name: "One container with schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"has_schedule_1"},
					ID:    "has_schedule_1",
					Labels: map[string]string{
						"dockron.schedule": "* * * * *",
					},
				},
			},
			expectedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1",
					Schedule:    "* * * * *",
					Context:     context.Background(),
					Client:      client,
				},
			},
		},
		{
			name: "One container with and one without schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"no_schedule_1"},
					ID:    "no_schedule_1",
				},
				dockerTypes.Container{
					Names: []string{"has_schedule_1"},
					ID:    "has_schedule_1",
					Labels: map[string]string{
						"dockron.schedule": "* * * * *",
					},
				},
			},
			expectedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1",
					Schedule:    "* * * * *",
					Context:     context.Background(),
					Client:      client,
				},
			},
		},
	}

	for loopIndex, c := range cases {
		t.Run(fmt.Sprintf("Loop %d: %s", loopIndex, c.name), func(t *testing.T) {
			log.Printf("Running %s", t.Name())

			// Load fake containers
			t.Logf("Fake containers: %+v", c.fakeContainers)
			client.FakeContainers = c.fakeContainers

			// Execute loop iteration loop
			// This is in the for loop
			croner := cron.New()
			jobs := QueryScheduledJobs(client)
			ScheduleJobs(croner, jobs)

			// Validate results

			scheduledEntries := croner.Entries()
			t.Logf("Cron entries: %+v", scheduledEntries)

			ErrorUnequal(t, len(c.expectedJobs), len(scheduledEntries), "Job and entry lengths don't match")
			for i, entry := range scheduledEntries {
				ErrorUnequal(t, c.expectedJobs[i], entry.Job, "Job value does not match entry")
			}
		})
	}

	// Make sure the cron stops
	croner.Stop()
}
