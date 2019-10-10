TOOLS ?= autonomy/toolchain:latest

# TODO(andrewrynhard): Move this logic to a shell script.
BUILDKIT_VERSION ?= v0.6.0
GO_VERSION ?= 1.12
BUILDKIT_IMAGE ?= moby/buildkit:$(BUILDKIT_VERSION)
BUILDKIT_HOST ?= tcp://0.0.0.0:1234
BUILDKIT_CONTAINER_NAME ?= talos-buildkit
BUILDKIT_CONTAINER_STOPPED := $(shell docker ps --filter name=$(BUILDKIT_CONTAINER_NAME) --filter status=exited --format='{{.Names}}' 2>/dev/null)
BUILDKIT_CONTAINER_RUNNING := $(shell docker ps --filter name=$(BUILDKIT_CONTAINER_NAME) --filter status=running --format='{{.Names}}' 2>/dev/null)

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
BUILDCTL_ARCHIVE := https://github.com/moby/buildkit/releases/download/$(BUILDKIT_VERSION)/buildkit-$(BUILDKIT_VERSION).linux-amd64.tar.gz
endif
ifeq ($(UNAME_S),Darwin)
BUILDCTL_ARCHIVE := https://github.com/moby/buildkit/releases/download/$(BUILDKIT_VERSION)/buildkit-$(BUILDKIT_VERSION).darwin-amd64.tar.gz
endif

ifeq ($(UNAME_S),Linux)
GITMETA := https://github.com/talos-systems/gitmeta/releases/download/v0.1.0-alpha.2/gitmeta-linux-amd64
endif
ifeq ($(UNAME_S),Darwin)
GITMETA := https://github.com/talos-systems/gitmeta/releases/download/v0.1.0-alpha.2/gitmeta-darwin-amd64
endif

BINDIR ?= ./bin
CONFORM_VERSION ?= 57c9dbd

SHA ?= $(shell $(BINDIR)/gitmeta git sha)
TAG ?= $(shell $(BINDIR)/gitmeta image tag)
BRANCH ?= $(shell $(BINDIR)/gitmeta git branch)

COMMON_ARGS = --progress=plain
COMMON_ARGS += --frontend=dockerfile.v0
COMMON_ARGS += --allow security.insecure
COMMON_ARGS += --local context=.
COMMON_ARGS += --local dockerfile=.
COMMON_ARGS += --opt build-arg:TOOLS=$(TOOLS)
COMMON_ARGS += --opt build-arg:SHA=$(SHA)
COMMON_ARGS += --opt build-arg:TAG=$(TAG)
COMMON_ARGS += --opt build-arg:GO_VERSION=$(GO_VERSION)

DOCKER_ARGS ?=

TESTPKGS ?= ./...

all: protoc-gen-proxy

.PHONY: ci
ci: builddeps buildkitd

.PHONY: builddeps
builddeps: gitmeta buildctl

gitmeta: $(BINDIR)/gitmeta

$(BINDIR)/gitmeta:
	@mkdir -p $(BINDIR)
	@curl -L $(GITMETA) -o $(BINDIR)/gitmeta
	@chmod +x $(BINDIR)/gitmeta

buildctl: $(BINDIR)/buildctl

$(BINDIR)/buildctl:
	@mkdir -p $(BINDIR)
	@curl -L $(BUILDCTL_ARCHIVE) | tar -zxf - -C $(BINDIR) --strip-components 1 bin/buildctl

.PHONY: buildkitd
buildkitd:
ifeq (tcp://0.0.0.0:1234,$(findstring tcp://0.0.0.0:1234,$(BUILDKIT_HOST)))
ifeq ($(BUILDKIT_CONTAINER_STOPPED),$(BUILDKIT_CONTAINER_NAME))
	@echo "Removing exited talos-buildkit container"
	@docker rm $(BUILDKIT_CONTAINER_NAME)
endif
ifneq ($(BUILDKIT_CONTAINER_RUNNING),$(BUILDKIT_CONTAINER_NAME))
	@echo "Starting talos-buildkit container"
	@docker run \
		--name $(BUILDKIT_CONTAINER_NAME) \
		-d \
		--privileged \
		-p 1234:1234 \
		$(BUILDKIT_IMAGE) \
		--addr $(BUILDKIT_HOST) \
		--allow-insecure-entitlement security.insecure
	@echo "Wait for buildkitd to become available"
	@sleep 5
endif
endif

protoc-gen-proxy: buildkitd
	@mkdir -p build
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=build/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

.PHONY: push
push: gitmeta login
	@docker push autonomy/protoc-gen-proxy:$(TAG)
ifeq ($(BRANCH),master)
	@docker tag autonomy/installer:$(TAG) autonomy/protoc-gen-proxy:latest
	@docker push autonomy/protoc-gen-proxy:latest
endif

.PHONY: unit-tests
unit-tests: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		--output type=local,dest=./ \
		--opt build-arg:TESTPKGS=$(TESTPKGS) \
		$(COMMON_ARGS)
