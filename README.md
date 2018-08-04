# Dockron

A Dead Simple Docker Cron Scheduler

## Usage

Dockron requires access to the Docker, so it may need to be run as root, or, if in a Docker container, need the socket mapped as a volume.

### Running Dockron

As simple as:

    dockron

It will then run in the foreground, periodically checking Docker for containers with labels containing a cron schedule.

By default, Dockron will periodically poll Docker for new containers or schedule changes every minute. You can specify an interval by using the `-watch` flag.

### Scheduling a container

First, be sure your container is something that is not long running and will actually exit when complete. This is for batch runs and not keeping a service running. Docker should be able to do that on it's own with a restart policy.

Create your container and add a label in the form `dockron.schedule="* * * * *"`, where the value is a valid cron expression (See the section [Cron Expression Formatting](#cron-expression-formatting)).

Dockron will now start that container peridically on the schedule.

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
