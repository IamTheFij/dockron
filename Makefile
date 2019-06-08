DOCKER_TAG ?= dockron-dev-${USER}

.PHONY: default
default: build

# Downloads dependencies into vendor directory
vendor:
	go mod vendor

# Runs the application, useful while developing
.PHONY: run
run:
	go run *.go

# Output target
dockron:
	go build -o dockron

# Alias for building
.PHONY: build
build: dockron

dockron-linux-amd64:
	GOARCH=amd64 CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o dockron-linux-amd64

dockron-linux-arm:
	GOARCH=arm CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o dockron-linux-arm

dockron-linux-arm64:
	GOARCH=arm64 CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o dockron-linux-arm64

.PHONY: build-all-static
build-all-static: dockron-linux-amd64 dockron-linux-arm dockron-linux-arm64

# Cleans all build artifacts
.PHONY: clean
clean:
	rm dockron
	rm dockron-linux-*

# Cleans vendor directory
.PHONY: clean-vendor
clean-vendor:
	rm -fr ./vendor

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
