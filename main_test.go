package main

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"testing"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/context"
)

var (
	// ContainerJSON results for a running container
	runningContainerInfo = dockerTypes.ContainerJSON{
		ContainerJSONBase: &dockerTypes.ContainerJSONBase{
			State: &dockerTypes.ContainerState{
				Running: true,
			},
		},
	}
	// ContainerJSON results for a stopped container
	stoppedContainerInfo = dockerTypes.ContainerJSON{
		ContainerJSONBase: &dockerTypes.ContainerJSONBase{
			State: &dockerTypes.ContainerState{
				Running: false,
			},
		},
	}
)

// FakeCall represents a faked method call
type FakeCall []interface{}

// FakeResult gives results of a fake method
type FakeResult []interface{}

// FakeDockerClient is used to test without interracting with Docker
type FakeDockerClient struct {
	FakeResults map[string][]FakeResult
	FakeCalls   map[string][]FakeCall
}

// AssertFakeCalls checks expected against actual calls to fake methods
func (fakeClient FakeDockerClient) AssertFakeCalls(t *testing.T, expectedCalls map[string][]FakeCall, message string) {
	if !reflect.DeepEqual(fakeClient.FakeCalls, expectedCalls) {
		t.Errorf(
			"%s: Expected and actual calls do not match. Expected %+v Actual %+v",
			message,
			expectedCalls,
			fakeClient.FakeCalls,
		)
	}
}

// called is a helper method to get return values and log the method call
func (fakeClient *FakeDockerClient) called(method string, v ...interface{}) FakeResult {
	if fakeClient.FakeCalls == nil {
		fakeClient.FakeCalls = map[string][]FakeCall{}
	}
	// Log method call
	fakeClient.FakeCalls[method] = append(fakeClient.FakeCalls[method], v)
	// Get fake results
	results := fakeClient.FakeResults[method][0]
	// Remove fake result
	fakeClient.FakeResults[method] = fakeClient.FakeResults[method][1:]
	// Return fake results
	return results
}

func (fakeClient *FakeDockerClient) ContainerStart(context context.Context, containerID string, options dockerTypes.ContainerStartOptions) (e error) {
	results := fakeClient.called("ContainerStart", context, containerID, options)
	if results[0] != nil {
		e = results[0].(error)
	}
	return
}

func (fakeClient *FakeDockerClient) ContainerList(context context.Context, options dockerTypes.ContainerListOptions) (c []dockerTypes.Container, e error) {
	results := fakeClient.called("ContainerList", context, options)
	if results[0] != nil {
		c = results[0].([]dockerTypes.Container)
	}
	if results[1] != nil {
		e = results[1].(error)
	}
	return
}

func (fakeClient *FakeDockerClient) ContainerExecCreate(ctx context.Context, container string, config dockerTypes.ExecConfig) (r dockerTypes.IDResponse, e error) {
	results := fakeClient.called("ContainerExecCreate", ctx, container, config)
	if results[0] != nil {
		r = results[0].(dockerTypes.IDResponse)
	}
	if results[1] != nil {
		e = results[1].(error)
	}
	return
}

func (fakeClient *FakeDockerClient) ContainerExecStart(ctx context.Context, execID string, config dockerTypes.ExecStartCheck) (e error) {
	results := fakeClient.called("ContainerExecStart", ctx, execID, config)
	if results[0] != nil {
		e = results[0].(error)
	}
	return
}

func (fakeClient *FakeDockerClient) ContainerExecInspect(ctx context.Context, execID string) (r dockerTypes.ContainerExecInspect, e error) {
	results := fakeClient.called("ContainerExecInspect", ctx, execID)
	if results[0] != nil {
		r = results[0].(dockerTypes.ContainerExecInspect)
	}
	if results[1] != nil {
		e = results[1].(error)
	}
	return
}

func (fakeClient *FakeDockerClient) ContainerInspect(ctx context.Context, containerID string) (r dockerTypes.ContainerJSON, e error) {
	results := fakeClient.called("ContainerInspect", ctx, containerID)
	if results[0] != nil {
		r = results[0].(dockerTypes.ContainerJSON)
	}
	if results[1] != nil {
		e = results[1].(error)
	}
	return
}

// NewFakeDockerClient creates an empty client
func NewFakeDockerClient() *FakeDockerClient {
	return &FakeDockerClient{
		FakeResults: map[string][]FakeResult{},
	}
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
			client.FakeResults["ContainerList"] = []FakeResult{
				FakeResult{c.fakeContainers, nil},
			}

			jobs := QueryScheduledJobs(client)
			// Sort so we can compare each list of jobs
			sort.Slice(jobs, func(i, j int) bool {
				return jobs[i].UniqueName() < jobs[j].UniqueName()
			})

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
			client.FakeResults["ContainerList"] = []FakeResult{
				FakeResult{c.fakeContainers, nil},
			}

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

// TestRunExecJobs does some verification on handling of exec jobs
// These tests aren't great because there are no return values to check
// but some test is better than no test! Future maybe these can be moved
// to a subpackage that offers a single function for interfacing with the
// Docker client to start or exec a container so that Dockron needn't care.
func TestRunExecJobs(t *testing.T) {

	var jobContext context.Context
	jobContainerID := "container_id"
	jobCommand := "true"

	cases := []struct {
		name          string
		client        *FakeDockerClient
		expectPanic   bool
		expectedCalls map[string][]FakeCall
	}{
		{
			name: "Initial inspect call raises error",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{nil, fmt.Errorf("error")},
					},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
			},
			expectPanic: true,
		},
		{
			name: "Handle container not running",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{stoppedContainerInfo, nil},
					},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
			},
		},
		{
			name: "Handle error creating exec",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{runningContainerInfo, nil},
					},
					"ContainerExecCreate": []FakeResult{
						FakeResult{nil, fmt.Errorf("fail")},
					},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
				"ContainerExecCreate": []FakeCall{
					FakeCall{
						jobContext,
						jobContainerID,
						dockerTypes.ExecConfig{
							Cmd: []string{"sh", "-c", jobCommand},
						},
					},
				},
			},
			expectPanic: true,
		},
		{
			name: "Fail starting exec container",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{runningContainerInfo, nil},
					},
					"ContainerExecCreate": []FakeResult{
						FakeResult{dockerTypes.IDResponse{ID: "id"}, nil},
					},
					"ContainerExecStart": []FakeResult{
						FakeResult{fmt.Errorf("fail")},
					},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
				"ContainerExecCreate": []FakeCall{
					FakeCall{
						jobContext,
						jobContainerID,
						dockerTypes.ExecConfig{
							Cmd: []string{"sh", "-c", jobCommand},
						},
					},
				},
				"ContainerExecStart": []FakeCall{
					FakeCall{jobContext, "id", dockerTypes.ExecStartCheck{}},
				},
			},
			expectPanic: true,
		},
		{
			name: "Successfully start an exec job fail on status",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{runningContainerInfo, nil},
					},
					"ContainerExecCreate": []FakeResult{
						FakeResult{dockerTypes.IDResponse{ID: "id"}, nil},
					},
					"ContainerExecStart": []FakeResult{
						FakeResult{nil},
					},
					"ContainerExecInspect": []FakeResult{
						FakeResult{nil, fmt.Errorf("fail")},
					},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
				"ContainerExecCreate": []FakeCall{
					FakeCall{
						jobContext,
						jobContainerID,
						dockerTypes.ExecConfig{
							Cmd: []string{"sh", "-c", jobCommand},
						},
					},
				},
				"ContainerExecStart": []FakeCall{
					FakeCall{jobContext, "id", dockerTypes.ExecStartCheck{}},
				},
				"ContainerExecInspect": []FakeCall{
					FakeCall{jobContext, "id"},
				},
			},
		},
		{
			name: "Successfully start an exec job and run to completion",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{runningContainerInfo, nil},
					},
					"ContainerExecCreate": []FakeResult{
						FakeResult{dockerTypes.IDResponse{ID: "id"}, nil},
					},
					"ContainerExecStart": []FakeResult{
						FakeResult{nil},
					},
					"ContainerExecInspect": []FakeResult{
						FakeResult{dockerTypes.ContainerExecInspect{Running: true}, nil},
						FakeResult{dockerTypes.ContainerExecInspect{Running: false}, nil},
					},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
				"ContainerExecCreate": []FakeCall{
					FakeCall{
						jobContext,
						jobContainerID,
						dockerTypes.ExecConfig{
							Cmd: []string{"sh", "-c", jobCommand},
						},
					},
				},
				"ContainerExecStart": []FakeCall{
					FakeCall{jobContext, "id", dockerTypes.ExecStartCheck{}},
				},
				"ContainerExecInspect": []FakeCall{
					FakeCall{jobContext, "id"},
					FakeCall{jobContext, "id"},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			log.Printf("Running %s", t.Name())

			// Create test job
			job := ContainerExecJob{
				ContainerStartJob: ContainerStartJob{
					name:        "test_job",
					context:     jobContext,
					client:      c.client,
					containerID: jobContainerID,
				},
				shellCommand: jobCommand,
			}

			defer func() {
				// Recover from panics, if there were any
				if err := recover(); err != nil {
					t.Log("Recovered from panic")
					t.Log(err)
				}
				c.client.AssertFakeCalls(t, c.expectedCalls, "Failed")
			}()
			job.Run()
			if c.expectPanic {
				t.Errorf("Expected panic but got none")
			}
		})
	}
}

// TestRunStartJobs does some verification on handling of start jobs
// These tests aren't great because there are no return values to check
// but some test is better than no test! Future maybe these can be moved
// to a subpackage that offers a single function for interfacing with the
// Docker client to start or exec a container so that Dockron needn't care.
func TestRunStartJobs(t *testing.T) {
	var jobContext context.Context
	jobContainerID := "container_id"

	cases := []struct {
		name          string
		client        *FakeDockerClient
		expectPanic   bool
		expectedCalls map[string][]FakeCall
	}{
		{
			name: "Initial inspect call raises error",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{nil, fmt.Errorf("error")},
					},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
			},
			expectPanic: true,
		},
		{
			name: "Handle container already running",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{runningContainerInfo, nil},
					},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
			},
		},
		{
			name: "Handle error starting container",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{stoppedContainerInfo, nil},
					},
					"ContainerStart": []FakeResult{FakeResult{fmt.Errorf("fail")}},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
				},
				"ContainerStart": []FakeCall{
					FakeCall{jobContext, jobContainerID, dockerTypes.ContainerStartOptions{}},
				},
			},
		},
		{
			name: "Succesfully start a container",
			client: &FakeDockerClient{
				FakeResults: map[string][]FakeResult{
					"ContainerInspect": []FakeResult{
						FakeResult{stoppedContainerInfo, nil},
						FakeResult{runningContainerInfo, nil},
						FakeResult{stoppedContainerInfo, nil},
					},
					"ContainerStart": []FakeResult{FakeResult{nil}},
				},
			},
			expectedCalls: map[string][]FakeCall{
				"ContainerInspect": []FakeCall{
					FakeCall{jobContext, jobContainerID},
					FakeCall{jobContext, jobContainerID},
					FakeCall{jobContext, jobContainerID},
				},
				"ContainerStart": []FakeCall{
					FakeCall{jobContext, jobContainerID, dockerTypes.ContainerStartOptions{}},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			log.Printf("Running %s", t.Name())

			// Create test job
			job := ContainerStartJob{
				name:        "test_job",
				context:     jobContext,
				client:      c.client,
				containerID: jobContainerID,
			}

			defer func() {
				// Recover from panics, if there were any
				recover()
				c.client.AssertFakeCalls(t, c.expectedCalls, "Failed")
			}()
			job.Run()
			if c.expectPanic {
				t.Errorf("Expected panic but got none")
			}

		})
	}
}
