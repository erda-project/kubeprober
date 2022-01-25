# syntax = docker/dockerfile:1.2

FROM golang:1.16 as builder

# specify the probe to build
ARG PROBER
# specify the apps under probe to build, all if not specified
ARG APPS
ENV PROBER=${PROBER}
ENV APPS=${APPS}
ENV GOPROXY=https://goproxy.cn,direct

COPY . /go/src/github.com/erda-project/kubeprober
WORKDIR /go/src/github.com/erda-project/kubeprober
RUN go mod tidy

# Build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd probers && \
    echo "workdir: $(pwd)"     && \
    echo "content: $(ls -lh)"  && \
    ./build.sh --prober=${PROBER} --apps=${APPS}

# no bash in image alpine, please use sh
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM kubeprober/probe-base:v0.1.6

COPY --from=builder /checkers checkers
COPY probers/run.sh /checkers/run.sh

WORKDIR /checkers
RUN chmod +x /checkers/run.sh

CMD [ "sh", "-c", "/checkers/run.sh"]

