DOCKER_TAG ?= dockron-dev

.PHONY: default
default: build

# Downloads dependencies into vendor directory
vendor:
	dep ensure

# Runs the application, useful while developing
.PHONY: run
run: vendor
	go run *.go

# Output target
dockron: vendor
	go build -o dockron

# Alias for building
.PHONY: build
build: dockron

# Cleans all build artifacts
.PHONY: clean
clean:
	rm dockron

# Cleans vendor directory
.PHONY: clean-vendor
clean-vendor:
	rm -fr ./vendor

# Attempts to update dependencies
.PHONY: dep-update
dep-update:
	dep ensure -update

.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_TAG) .

.PHONY: docker-run
docker-run:
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --name $(DOCKER_TAG)-run $(DOCKER_TAG)
