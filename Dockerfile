# syntax = docker/dockerfile-upstream:1.1.2-experimental

# Meta args applied to stage base names.

ARG TOOLS
ARG GO_VERSION

# The tools target provides base toolchain for the build.

FROM $TOOLS AS tools
ENV PATH /toolchain/bin:/toolchain/go/bin
RUN ["/toolchain/bin/mkdir", "-p", "/bin", "/tmp"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/bin/bash", "/bin/sh"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/etc/ssl", "/etc/ssl"]
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | \
    bash -s -- -b /toolchain/bin v1.20.0
RUN cd $(mktemp -d) \
    && go mod init tmp \
    && go get mvdan.cc/gofumpt/gofumports \
    && mv /go/bin/gofumports /toolchain/go/bin/gofumports
RUN curl -sfL https://github.com/uber/prototool/releases/download/v1.8.0/prototool-Linux-x86_64.tar.gz | \
    tar -xz --strip-components=2 -C /toolchain/bin prototool/bin/prototool

# The build target creates a container that will be used to build the source
# code.

FROM scratch AS build
COPY --from=tools / /
SHELL ["/toolchain/bin/bash", "-c"]
ENV PATH /toolchain/bin:/toolchain/go/bin
ENV GO111MODULE on
ENV GOPROXY https://proxy.golang.org
ENV CGO_ENABLED 0
WORKDIR /src

FROM build AS base
COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download
RUN go mod verify
COPY ./pkg ./pkg
COPY ./main.go ./main.go
RUN go list -mod=readonly all >/dev/null
RUN ! go mod tidy -v 2>&1 | grep .

FROM base AS protoc-gen-proxy-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src
RUN --mount=type=cache,target=/.cache/go-build \
    go build -ldflags \
    "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" \
    -o /protoc-gen-proxy
RUN chmod +x /protoc-gen-proxy

FROM scratch AS protoc-gen-proxy
COPY --from=protoc-gen-proxy-build /protoc-gen-proxy /protoc-gen-proxy

#FROM base AS unit-tests-runner
#RUN unlink /etc/ssl
#ARG TESTPKGS
#RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache/go-build \
#    go test -v
#
#FROM scratch AS unit-tests
#COPY --from=unit-tests-runner /src/coverage.txt /coverage.txt
