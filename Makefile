PROJECT = github.com/timelinelabs/romulus
EXECUTABLE = "romulus"
LDFLAGS = "-s"
TEST_COMMAND = godep go test
PLATFORM = "$$(echo "$$(uname)" | tr '[A-Z]' '[a-z]')"
VERSION = "$$(./t2 -v)"
BUILD_ARGS = ""

.PHONY: dep-save dep-restore test test-verbose build install clean

all: test

help:
	@echo "Available targets:"
	@echo ""
	@echo "  dep-save"
	@echo "  dep-restore"
	@echo "  test"
	@echo "  test-verbose"
	@echo "  build"
	@echo "  build-docker"
	@echo "  install"
	@echo "  clean"

dep-save:
	godep save ./...

dep-restore:
	godep restore

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
	@godep go build -ldflags $(LDFLAGS)

install:
	@echo "==> Installing $(EXECUTABLE) with ldflags $(LDFLAGS)"
	@godep go install -ldflags $(LDFLAGS) $(INSTALL)

package: build
	@echo "==> Tar'ing up the binary"
	@test -f escarole && tar czf escarole-$(PLATFORM).tar.gz escarole

clean:
	go clean ./...
	rm -rf escarole *.tar.gz
