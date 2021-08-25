# syntax = docker/dockerfile:1.2

# Build the manager binary
FROM golang:1.16 as builder

ARG APP
WORKDIR /workspace
ENV APP=${APP}
ENV GOPROXY=https://goproxy.cn,direct

COPY . /workspace
# Build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod readonly -a -o ${APP} ./cmd/${APP}/${APP}.go

FROM centos:7
ARG APP
ENV APP=${APP}
WORKDIR /

COPY --from=builder /workspace/${APP} .

CMD [ "sh", "-c", "/${APP}"]