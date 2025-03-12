# Build the manager binary
FROM golang:1.22 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace/module-controller
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY common/ common/
COPY controller/ controller/
COPY module_tunnels/ module_tunnels/
COPY report_server/ report_server/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o module_controller cmd/module-controller/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM golang:1.22.8
WORKDIR /
COPY config/ config/
COPY --from=builder /workspace/module-controller/module_controller .

RUN git clone -b v1.24.1 https://github.com/go-delve/delve
RUN cd delve && go mod download
RUN cd delve && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o dlv cmd/dlv/main.go
RUN cp delve/dlv /usr/local/bin/dlv

EXPOSE 9090
EXPOSE 8080
EXPOSE 7777
EXPOSE 2345

#ENTRYPOINT ["dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./module_controller"]
CMD ["/bin/sh", "-c", "tail -f /dev/null"]