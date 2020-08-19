# Dockron

Simple scheduling for short-running Docker containers

## Usage

Dockron requires access to the Docker, so it may need to be run as root, or, if in a Docker container, need the socket mapped as a volume.

### Running Dockron

As simple as:

    dockron

It will then run in the foreground, periodically checking Docker for containers with labels containing a cron schedule.

By default, Dockron will periodically poll Docker for new containers or schedule changes every minute. You can specify an interval by using the `-watch` flag.

### Running with Docker

Dockron is also available as a Docker image. The multi-arch repo can be found at [IamTheFij/dockron](https://hub.docker.com/r/iamthefij/dockron)

From either an `amd64`, `arm`, or `arm64` machine, you can run Dockron using:

    docker run -v /var/run/docker.sock:/var/run/docker.sock:ro iamthefij/dockron -watch

### Scheduling a container

First, be sure your container is something that is not long running and will actually exit when complete. This is for batch runs and not keeping a service running. Docker should be able to do that on it's own with a restart policy.

Create your container and add a label in the form `'dockron.schedule=* * * * *'`, where the value is a valid cron expression (See the section [Cron Expression Formatting](#cron-expression-formatting)).

Dockron will now start that container peridically on the schedule.

If you have a long running container that you'd like to schedule an exec command inside of, you can do so with labels as well. Add your job in the form `dockron.<job>.schedule=* * * * *` and `dockeron.<job>.command=echo hello`. Both labels are required to create an exec job.

Eg.

    labels:
        - "dockron.dates.schedule=* * * * *"
        - "dockron.dates.command=date"

_Note: Exec jobs will not log their output anywhere. Not to the host container or to Dockron. It's up to you to deal with this for now. There is also currently no way to health check these._

### Cron Expression Formatting

For more information on the cron expression parsing, see the docs for [robfig/cron](https://godoc.org/github.com/robfig/cron).

## Caveats

Dockron is quite simple right now. It does not yet:

* Issue any retries
* Cancel hanging jobs

I intend to keep it simple as well. It will likely never:

* Provide any kind of alerting (check out [Minitor](https://git.iamthefij.com/IamTheFij/minitor))
* Handle job dependencies

Either use a separate tool in conjunction with Dockron, or use a more robust scheduler like Tron, or Chronos.

## Building

If you have go on your machine, you can simply use `make build` or `make run` to build and test Dockron. If you don't have go but you do have Docker, you can still build docker images using the provide multi-stage Dockerfile! You can kick that off with `make docker-staged-build`

There is also an example `docker-compose.yml` that will use the multi-stage build to ensure an easy sample. This can be run with `make docker-example`.

## Tests

There are now some basic tests as well as linting and integration tests. You can run all of these by executing `make all`.
