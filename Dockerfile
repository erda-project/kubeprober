
# Build the manager binary
FROM golang:1.16 as builder

ARG APP_NAME
WORKDIR /workspace
ENV APP_NAME=${APP_NAME}
ENV GOPROXY=https://goproxy.cn

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
# Copy the go source

COPY apistructs/ apistructs/
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o ${APP_NAME} ./cmd/${APP_NAME}/${APP_NAME}.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.9
ARG APP_NAME
ENV APP_NAME=${APP_NAME}
WORKDIR /workspace

COPY --from=builder /workspace/${APP_NAME} /workspace
#USER 65532:65532

ENTRYPOINT [ "sh", "-c", "/workspace/${APP_NAME}"]
