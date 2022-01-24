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
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod readonly -a  -o ${APP} ./cmd/${APP}/${APP}.go

#FROM centos:7
FROM kubeprober/alpine:v3.9

ARG ARCH=amd64
ARG APP
ENV KUBECTL_VERSION v1.19.7
ENV APP=${APP}
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib:/usr/local/lib64:/lib:/lib64

WORKDIR /

COPY --from=builder /workspace/scripts/shell-setup.sh /usr/bin/shell-setup.sh
COPY --from=builder /workspace/scripts/kubectl-shell.sh /usr/bin/kubectl-shell.sh

RUN apk add --no-cache tzdata jq bash curl libcurl shadow bash-completion

RUN if [ "$APP" = "probe-agent" ] ; then \
    curl -sLf https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl > /usr/bin/kubectl && \
    chmod +x /usr/bin/kubectl /usr/bin/kubectl-shell.sh /usr/bin/shell-setup.sh ; \
    fi

COPY --from=builder /workspace/${APP} .

CMD [ "sh", "-c", "/${APP}"]