DOCKER_TAG ?= dockron-dev-${USER}
GIT_TAG_NAME := $(shell git tag -l --contains HEAD)
GIT_SHA := $(shell git rev-parse HEAD)
VERSION := $(if $(GIT_TAG_NAME),$(GIT_TAG_NAME),$(GIT_SHA))

.PHONY: default
default: build

# Downloads dependencies into vendor directory
vendor:
	go mod vendor

# Runs the application, useful while developing
.PHONY: run
run:
	go run .

.PHONY: test
test:
	go test -coverprofile=coverage.out
	go tool cover -func=coverage.out
	# @go tool cover -func=coverage.out | awk -v target=80.0% \
		'/^total:/ { print "Total coverage: " $$3 " Minimum coverage: " target; if ($$3+0.0 >= target+0.0) print "ok"; else { print "fail"; exit 1; } }'

# Output target
dockron:
	@echo Version: $(VERSION)
	go build -ldflags '-X "main.version=${VERSION}"' -o dockron

# Alias for building
.PHONY: build
build: dockron

dockron-darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
		   go build -ldflags '-X "main.version=${VERSION}"' -a -installsuffix nocgo \
		   -o dockron-darwin-amd64

dockron-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		   go build -ldflags '-X "main.version=${VERSION}"' -a -installsuffix nocgo \
		   -o dockron-linux-amd64

dockron-linux-arm:
	GOOS=linux GOARCH=arm CGO_ENABLED=0 \
		   go build -ldflags '-X "main.version=${VERSION}"' -a -installsuffix nocgo \
		   -o dockron-linux-arm

dockron-linux-arm64:
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
	rm -f dockron
	rm -f dockron-linux-*

# Cleans vendor directory
.PHONY: clean-vendor
clean-vendor:
	rm -fr ./vendor

.PHONY: docker-build
docker-build: dockron-linux-amd64
	docker build . -t ${DOCKER_TAG}-linux-amd64

# Cross build for arm architechtures
.PHONY: docker-build-arm
docker-build-arm: dockron-linux-arm
	docker build --build-arg REPO=arm32v7 --build-arg ARCH=arm . -t ${DOCKER_TAG}-linux-arm

.PHONY: docker-build-arm
docker-build-arm64: dockron-linux-arm64
	docker build --build-arg REPO=arm64v8 --build-arg ARCH=arm64 . -t ${DOCKER_TAG}-linux-arm64

.PHONY: docker-run
docker-run: docker-build
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --name $(DOCKER_TAG)-run $(DOCKER_TAG)-linux-amd64

# Cross run on host architechture
.PHONY: docker-run-arm
docker-run-arm: docker-build-arm
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --name $(DOCKER_TAG)-run ${DOCKER_TAG}-linux-arm

.PHONY: docker-run-arm64
docker-run-arm64: docker-build-arm64
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --name $(DOCKER_TAG)-run ${DOCKER_TAG}-linux-arm64

# Multi stage builds
.PHONY: docker-staged-build
docker-staged-build:
	docker build --build-arg VERSION=${VERSION} \
		-t ${DOCKER_TAG}-linux-amd64 \
		-f Dockerfile.multi-stage .

# Cross build for arm architechtures
.PHONY: docker-staged-build-arm
docker-staged-build-arm:
	docker build --build-arg VERSION=${VERSION} \
		--build-arg REPO=arm32v7 --build-arg ARCH=arm -t ${DOCKER_TAG}-linux-arm \
		-f Dockerfile.multi-stage .

.PHONY: docker-staged-build-arm
docker-staged-build-arm64:
	docker build --build-arg VERSION=${VERSION} \
		--build-arg REPO=arm64v8 --build-arg ARCH=arm64 -t ${DOCKER_TAG}-linux-arm64 \
		-f Dockerfile.multi-stage .
