PROJECT = github.com/timelinelabs/romulus
EXECUTABLE = "romulusd"
BINARY = cmd/romulusd/romulusd.go
LDFLAGS = "-s"
TEST_COMMAND = godep go test

.PHONY: dep-save dep-restore test test-verbose build build-image install

all: test build build-image

help:
	@echo "Available targets:"
	@echo ""
	@echo "  dep-save"
	@echo "  dep-restore"
	@echo "  test"
	@echo "  test-verbose"
	@echo "  build"
	@echo "  build-image"
	@echo "  install"

dep-save:
	@echo "==> Saving dependencies to ./Godeps"
	@godep save ./...

dep-restore:
	@echo "==> Restoring dependencies from ./Godeps"
	@godep restore

test:
	@echo "==> Running all tests"
	@echo ""
	@$(TEST_COMMAND) ./...

test-verbose:
	@echo "==> Running all tests (verbose output)"
	@ echo ""
	@$(TEST_COMMAND) -test.v ./...

build:
	@echo "==> Building $(EXECUTABLE) with ldflags '$(LDFLAGS)'"
	@godep go build -ldflags $(LDFLAGS) -o bin/romulusd $(BINARY)

build-image: bin/romulusd-linux
	@echo "==> Building linux binary"
	@ GOOS=linux CGO_ENABLED=0 godep go build -a -installsuffix cgo -ldflags $(LDFLAGS) -o bin/romulusd-linux $(BINARY)
	@echo "==> Building docker image 'romulusd'"
	@docker build -t romulusd .

install:
	@echo "==> Installing $(EXECUTABLE) with ldflags '$(LDFLAGS)'"
	@godep go install -ldflags $(LDFLAGS) $(BINARY)
