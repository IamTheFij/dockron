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

// TestScheduleJobs validates that only new jobs get created
func TestScheduleJobs(t *testing.T) {
	croner := cron.New()

	// Each cases is on the same cron instance
	// Tests must be executed sequentially!
	cases := []struct {
		name         string
		queriedJobs  []ContainerStartJob
		expectedJobs []ContainerStartJob
	}{
		{
			name:         "No containers",
			queriedJobs:  []ContainerStartJob{},
			expectedJobs: []ContainerStartJob{},
		},
		{
			name: "One container with schedule",
			queriedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1",
					Schedule:    "* * * * *",
				},
			},
			expectedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1",
					Schedule:    "* * * * *",
				},
			},
		},
		{
			name: "Add a second job",
			queriedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1",
					Schedule:    "* * * * *",
				},
				ContainerStartJob{
					Name:        "has_schedule_2",
					ContainerID: "has_schedule_2",
					Schedule:    "* * * * *",
				},
			},
			expectedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1",
					Schedule:    "* * * * *",
				},
				ContainerStartJob{
					Name:        "has_schedule_2",
					ContainerID: "has_schedule_2",
					Schedule:    "* * * * *",
				},
			},
		},
		{
			name: "Replace job 1",
			queriedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1_prime",
					Schedule:    "* * * * *",
				},
				ContainerStartJob{
					Name:        "has_schedule_2",
					ContainerID: "has_schedule_2",
					Schedule:    "* * * * *",
				},
			},
			expectedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_2",
					ContainerID: "has_schedule_2",
					Schedule:    "* * * * *",
				},
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1_prime",
					Schedule:    "* * * * *",
				},
			},
		},
	}

	for loopIndex, c := range cases {
		t.Run(fmt.Sprintf("Loop %d: %s", loopIndex, c.name), func(t *testing.T) {
			log.Printf("Running %s", t.Name())

			t.Logf("Expected jobs: %+v Queried jobs: %+v", c.expectedJobs, c.queriedJobs)

			ScheduleJobs(croner, c.queriedJobs)

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

// TestDoLoop is close to an integration test that checks the main loop logic
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
		{
			name: "Add a second container with a schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"has_schedule_1"},
					ID:    "has_schedule_1",
					Labels: map[string]string{
						"dockron.schedule": "* * * * *",
					},
				},
				dockerTypes.Container{
					Names: []string{"has_schedule_2"},
					ID:    "has_schedule_2",
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
				ContainerStartJob{
					Name:        "has_schedule_2",
					ContainerID: "has_schedule_2",
					Schedule:    "* * * * *",
					Context:     context.Background(),
					Client:      client,
				},
			},
		},
		{
			name: "Modify the first container",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"has_schedule_1"},
					ID:    "has_schedule_1_prime",
					Labels: map[string]string{
						"dockron.schedule": "* * * * *",
					},
				},
				dockerTypes.Container{
					Names: []string{"has_schedule_2"},
					ID:    "has_schedule_2",
					Labels: map[string]string{
						"dockron.schedule": "* * * * *",
					},
				},
			},
			expectedJobs: []ContainerStartJob{
				ContainerStartJob{
					Name:        "has_schedule_2",
					ContainerID: "has_schedule_2",
					Schedule:    "* * * * *",
					Context:     context.Background(),
					Client:      client,
				},
				ContainerStartJob{
					Name:        "has_schedule_1",
					ContainerID: "has_schedule_1_prime",
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
