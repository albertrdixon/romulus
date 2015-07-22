PROJECT = github.com/timelinelabs/romulus
REV = $$(git rev-parse --short=8 HEAD)
BRANCH = $$(git rev-parse --abbrev-ref HEAD | tr / _)
EXECUTABLE = "romulusd"
BINARY = cmd/romulusd/romulusd.go
IMAGE = romulusd
REMOTE_REPO = quay.io/timeline_labs/romulusd
LDFLAGS = "-s -X $(PROJECT)/romulus.SHA $(REV)"
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
	@godep go build -ldflags $(LDFLAGS) -o bin/$(EXECUTABLE) $(BINARY)

build-image:
	@echo "==> Building linux binary"
	@ GOOS=linux CGO_ENABLED=0 godep go build -a -installsuffix cgo -ldflags $(LDFLAGS) -o bin/$(EXECUTABLE)-linux $(BINARY)
	@echo "==> Building docker image '$(IMAGE)'"
	@docker build -t $(IMAGE) .

publish:
	@echo "==> Publishing $(EXECUTABLE) to $(REMOTE_REPO)"
	@echo "==> Tagging with '$(BRANCH)' and pushing"
	@docker rmi $(REMOTE_REPO):$(BRANCH) >/dev/null 2>&1 || true
	@docker tag $(IMAGE) $(REMOTE_REPO):$(BRANCH)
	@docker push $(REMOTE_REPO):$(BRANCH)
	@echo "==> Tagging with '$(REV)' and pushing"
	@docker rmi $(REMOTE_REPO):$(REV) >/dev/null 2>&1 || true
	@docker tag $(IMAGE) $(REMOTE_REPO):$(REV)
	@docker push $(REMOTE_REPO):$(REV)

install:
	@echo "==> Installing $(EXECUTABLE) with ldflags '$(LDFLAGS)'"
	@godep go install -ldflags $(LDFLAGS) $(BINARY)
