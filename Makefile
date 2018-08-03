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
dsched: vendor
	go build -o dsched

# Alias for building
.PHONY: build
build: dsched

# Cleans all build artifacts
.PHONY: clean
clean:
	rm dsched

# Cleans vendor directory
.PHONY: clean-vendor
clean-vendor:
	rm -fr ./vendor

# Attempts to update dependencies
.PHONY: dep-update
dep-update:
	dep ensure -update
