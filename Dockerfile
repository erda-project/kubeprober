
# Build the manager binary
FROM golang:1.16 as builder

ARG APP
WORKDIR /workspace
ENV APP=${APP}
ENV GOPROXY=https://goproxy.cn,direct

COPY . /workspace
# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod readonly -a -o ${APP} ./cmd/${APP}/${APP}.go

FROM centos:7
ARG APP
ENV APP=${APP}
WORKDIR /

COPY --from=builder /workspace/${APP} .

CMD [ "sh", "-c", "/${APP}"]