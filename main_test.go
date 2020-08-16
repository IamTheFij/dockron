package main

import (
	"fmt"
	"log"
	"sort"
	"testing"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/context"
)

// FakeDockerClient is used to test without interracting with Docker
type FakeDockerClient struct {
	FakeContainers           []dockerTypes.Container
	FakeExecIDResponse       string
	FakeContainerExecInspect dockerTypes.ContainerExecInspect
	FakeContainerInspect     dockerTypes.ContainerJSON
}

// ContainerStart pretends to start a container
func (fakeClient *FakeDockerClient) ContainerStart(context context.Context, containerID string, options dockerTypes.ContainerStartOptions) error {
	return nil
}

func (fakeClient *FakeDockerClient) ContainerList(context context.Context, options dockerTypes.ContainerListOptions) ([]dockerTypes.Container, error) {
	return fakeClient.FakeContainers, nil
}

func (fakeClient *FakeDockerClient) ContainerExecCreate(ctx context.Context, container string, config dockerTypes.ExecConfig) (dockerTypes.IDResponse, error) {
	return dockerTypes.IDResponse{ID: fakeClient.FakeExecIDResponse}, nil
}

func (fakeClient *FakeDockerClient) ContainerExecStart(ctx context.Context, execID string, config dockerTypes.ExecStartCheck) error {
	return nil
}

func (fakeClient *FakeDockerClient) ContainerExecInspect(ctx context.Context, execID string) (dockerTypes.ContainerExecInspect, error) {
	return fakeClient.FakeContainerExecInspect, nil
}

func (fakeClient *FakeDockerClient) ContainerInspect(ctx context.Context, containerID string) (dockerTypes.ContainerJSON, error) {
	return fakeClient.FakeContainerInspect, nil
}

// newFakeDockerClient creates an empty client
func newFakeDockerClient() *FakeDockerClient {
	return &FakeDockerClient{}
}

// errorUnequal checks that two values are equal and fails the test if not
func errorUnequal(t *testing.T, expected interface{}, actual interface{}, message string) {
	if expected != actual {
		t.Errorf("%s Expected: %+v Actual: %+v", message, expected, actual)
	}
}

// TestQueryScheduledJobs checks that when querying the Docker client that we
// create jobs for any containers with a dockron.schedule
func TestQueryScheduledJobs(t *testing.T) {
	client := newFakeDockerClient()

	cases := []struct {
		name           string
		fakeContainers []dockerTypes.Container
		expectedJobs   []ContainerCronJob
	}{
		{
			name:           "No containers",
			fakeContainers: []dockerTypes.Container{},
			expectedJobs:   []ContainerCronJob{},
		},
		{
			name: "One container without schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"no_schedule_1"},
					ID:    "no_schedule_1",
				},
			},
			expectedJobs: []ContainerCronJob{},
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
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
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
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
				},
			},
		},
		{
			name: "Incomplete exec job, schedule only",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"exec_job_1"},
					ID:    "exec_job_1",
					Labels: map[string]string{
						"dockron.test.schedule": "* * * * *",
					},
				},
			},
			expectedJobs: []ContainerCronJob{},
		},
		{
			name: "Incomplete exec job, command only",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"exec_job_1"},
					ID:    "exec_job_1",
					Labels: map[string]string{
						"dockron.test.command": "date",
					},
				},
			},
			expectedJobs: []ContainerCronJob{},
		},
		{
			name: "Complete exec job",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"exec_job_1"},
					ID:    "exec_job_1",
					Labels: map[string]string{
						"dockron.test.schedule": "* * * * *",
						"dockron.test.command":  "date",
					},
				},
			},
			expectedJobs: []ContainerCronJob{
				ContainerExecJob{
					ContainerStartJob: ContainerStartJob{
						name:        "exec_job_1/test",
						containerID: "exec_job_1",
						schedule:    "* * * * *",
						context:     context.Background(),
						client:      client,
					},
					shellCommand: "date",
				},
			},
		},
		{
			name: "Dual exec jobs on single container",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"exec_job_1"},
					ID:    "exec_job_1",
					Labels: map[string]string{
						"dockron.test1.schedule": "* * * * *",
						"dockron.test1.command":  "date",
						"dockron.test2.schedule": "* * * * *",
						"dockron.test2.command":  "date",
					},
				},
			},
			expectedJobs: []ContainerCronJob{
				ContainerExecJob{
					ContainerStartJob: ContainerStartJob{
						name:        "exec_job_1/test1",
						containerID: "exec_job_1",
						schedule:    "* * * * *",
						context:     context.Background(),
						client:      client,
					},
					shellCommand: "date",
				},
				ContainerExecJob{
					ContainerStartJob: ContainerStartJob{
						name:        "exec_job_1/test2",
						containerID: "exec_job_1",
						schedule:    "* * * * *",
						context:     context.Background(),
						client:      client,
					},
					shellCommand: "date",
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
			// Sort so we can compare each list of jobs
			sort.Slice(jobs, func(i, j int) bool {
				return jobs[i].UniqueName() < jobs[j].UniqueName()
			})

			t.Logf("Expected jobs: %+v, Actual jobs: %+v", c.expectedJobs, jobs)
			errorUnequal(t, len(c.expectedJobs), len(jobs), "Job lengths don't match")
			for i, job := range jobs {
				errorUnequal(t, c.expectedJobs[i], job, "Job value does not match")
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
		queriedJobs  []ContainerCronJob
		expectedJobs []ContainerCronJob
	}{
		{
			name:         "No containers",
			queriedJobs:  []ContainerCronJob{},
			expectedJobs: []ContainerCronJob{},
		},
		{
			name: "One container with schedule",
			queriedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
				},
			},
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
				},
			},
		},
		{
			name: "Add a second job",
			queriedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
				},
				ContainerStartJob{
					name:        "has_schedule_2",
					containerID: "has_schedule_2",
					schedule:    "* * * * *",
				},
			},
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
				},
				ContainerStartJob{
					name:        "has_schedule_2",
					containerID: "has_schedule_2",
					schedule:    "* * * * *",
				},
			},
		},
		{
			name: "Replace job 1",
			queriedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1_prime",
					schedule:    "* * * * *",
				},
				ContainerStartJob{
					name:        "has_schedule_2",
					containerID: "has_schedule_2",
					schedule:    "* * * * *",
				},
			},
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_2",
					containerID: "has_schedule_2",
					schedule:    "* * * * *",
				},
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1_prime",
					schedule:    "* * * * *",
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

			errorUnequal(t, len(c.expectedJobs), len(scheduledEntries), "Job and entry lengths don't match")
			for i, entry := range scheduledEntries {
				errorUnequal(t, c.expectedJobs[i], entry.Job, "Job value does not match entry")
			}
		})
	}

	// Make sure the cron stops
	croner.Stop()
}

// TestDoLoop is close to an integration test that checks the main loop logic
func TestDoLoop(t *testing.T) {
	croner := cron.New()
	client := newFakeDockerClient()

	cases := []struct {
		name           string
		fakeContainers []dockerTypes.Container
		expectedJobs   []ContainerCronJob
	}{
		{
			name:           "No containers",
			fakeContainers: []dockerTypes.Container{},
			expectedJobs:   []ContainerCronJob{},
		},
		{
			name: "One container without schedule",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"no_schedule_1"},
					ID:    "no_schedule_1",
				},
			},
			expectedJobs: []ContainerCronJob{},
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
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
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
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
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
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
				},
				ContainerStartJob{
					name:        "has_schedule_2",
					containerID: "has_schedule_2",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
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
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_2",
					containerID: "has_schedule_2",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
				},
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1_prime",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
				},
			},
		},
		{
			name: "Remove second container and add exec to first",
			fakeContainers: []dockerTypes.Container{
				dockerTypes.Container{
					Names: []string{"has_schedule_1"},
					ID:    "has_schedule_1_prime",
					Labels: map[string]string{
						"dockron.schedule":      "* * * * *",
						"dockron.test.schedule": "* * * * *",
						"dockron.test.command":  "date",
					},
				},
			},
			expectedJobs: []ContainerCronJob{
				ContainerStartJob{
					name:        "has_schedule_1",
					containerID: "has_schedule_1_prime",
					schedule:    "* * * * *",
					context:     context.Background(),
					client:      client,
				},
				ContainerExecJob{
					ContainerStartJob: ContainerStartJob{
						name:        "has_schedule_1/test",
						containerID: "has_schedule_1_prime",
						schedule:    "* * * * *",
						context:     context.Background(),
						client:      client,
					},
					shellCommand: "date",
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

			errorUnequal(t, len(c.expectedJobs), len(scheduledEntries), "Job and entry lengths don't match")
			for i, entry := range scheduledEntries {
				errorUnequal(t, c.expectedJobs[i], entry.Job, "Job value does not match entry")
			}
		})
	}

	// Make sure the cron stops
	croner.Stop()
}
