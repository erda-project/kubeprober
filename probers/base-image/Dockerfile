#
## Build the manager binary
#FROM golang:1.16 as builder
#
#ARG APP
## WORKDIR /go/src/github.com/erda-project/kubeprober
#WORKDIR /workspace
#ENV GOPROXY=https://goproxy.cn,direct
#
#COPY . /workspace
## Build
##RUN --mount=type=cache,target=/root/.cache/go-build\
##    --mount=type=cache,target=/go/pkg/mod \
##    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod readonly -a -o main ./main.go
#
## Use distroless as minimal base image to package the manager binary
## Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM centos:7
WORKDIR /

COPY main.sh .
COPY bin/kubectl /bin
COPY bin/report-status /bin
RUN mkdir /checker
#USER 65532:65532

CMD [ "sh", "-c", "/main.sh"]