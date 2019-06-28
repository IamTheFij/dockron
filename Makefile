DOCKER_TAG ?= dockron-dev-${USER}
GIT_TAG_NAME := $(shell git tag -l --contains HEAD)
GIT_SHA := $(shell git rev-parse HEAD)
VERSION := $(if $(GIT_TAG_NAME),$(GIT_TAG_NAME),$(GIT_SHA))

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
	@echo Version: $(VERSION)
	go build -ldflags '-X "main.version=${VERSION}"' -o dockron

# Alias for building
.PHONY: build
build: dockron

dockron-darwin-amd64: vendor
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
		   go build -ldflags '-X "main.version=${VERSION}"' -a -installsuffix nocgo \
		   -o dockron-darwin-amd64

dockron-linux-amd64: vendor
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		   go build -ldflags '-X "main.version=${VERSION}"' -a -installsuffix nocgo \
		   -o dockron-linux-amd64

dockron-linux-arm: vendor
	GOOS=linux GOARCH=arm CGO_ENABLED=0 \
		   go build -ldflags '-X "main.version=${VERSION}"' -a -installsuffix nocgo \
		   -o dockron-linux-arm

dockron-linux-arm64: vendor
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		   go build -ldflags '-X "main.version=${VERSION}"' -a -installsuffix nocgo \
		   -o dockron-linux-arm64

.PHONY: build-linux-static
build-linux-static: dockron-linux-amd64 dockron-linux-arm dockron-linux-arm64

.PHONY: build-all-static
build-all-static: dockron-darwin-amd64 build-linux-static

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
	docker build . -t ${DOCKER_TAG}-linux-amd64

# Cross build for arm architechtures
.PHONY: docker-cross-build-arm
docker-cross-build-arm:
	docker build --build-arg REPO=arm32v6 --build-arg ARCH=arm . -t ${DOCKER_TAG}-linux-arm

.PHONY: docker-cross-build-arm
docker-cross-build-arm64:
	docker build --build-arg REPO=arm64v8 --build-arg ARCH=arm64 . -t ${DOCKER_TAG}-linux-arm64

.PHONY: docker-run
docker-run: docker-build
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --name $(DOCKER_TAG)-run $(DOCKER_TAG)-linux-amd64

# Cross run on host architechture
.PHONY: docker-cross-run-arm
docker-cross-run-arm: docker-cross-build-arm
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --name $(DOCKER_TAG)-run ${DOCKER_TAG}-linux-arm

.PHONY: docker-cross-run-arm64
docker-cross-run-arm64: docker-cross-build-arm64
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --name $(DOCKER_TAG)-run ${DOCKER_TAG}-linux-arm64
