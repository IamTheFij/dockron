#! /bin/bash

set -e

# Change to itest dir
cd "$(dirname "$0")"

function check_results() {
    local f=$1
    local min=$2
    awk "/ok/ { count=count+1 } END { print \"$f: Run count\", count; if (count < $min) { print \"Expected > $min\"; exit 1 } }" "$f"
}

function main() {
    # Clear and create result files
    echo "start" > ./start_result.txt
    echo "start" > ./exec_result.txt

    # Clean old containers
    docker compose down || true
    # Start containers
    echo "Starting containers"
    docker compose up -d --build
    # Schedules run on the shortest interval of a minute. This should allow time
    # for the containers to start and execute once
    local seconds=$((65 - $(date +"%S")))
    echo "Containers started. Sleeping for ${seconds}s to let schedules run"
    sleep $seconds

    echo "Stopping containers"
    docker compose stop

    # Validate result shows minimum amount of executions
    check_results ./start_result.txt 2
    check_results ./exec_result.txt 1
}

main
