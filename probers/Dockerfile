
# Build the manager binary
FROM golang:1.16 as builder

ARG APP
# WORKDIR /go/src/github.com/erda-project/kubeprober
ENV APP=${APP}
ENV GOPROXY=https://goproxy.cn,direct

COPY . /workspace
WORKDIR /workspace/probers/${APP}

# Build
RUN pwd && \
    ls -lh && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod readonly -a -o ${APP} ./main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM scratch
ARG APP
ENV APP=${APP}
WORKDIR /

COPY --from=builder /workspace/probers/${APP}/${APP} .
#USER 65532:65532

CMD [ "sh", "-c", "/${APP}"]


