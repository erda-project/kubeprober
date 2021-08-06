# lastet version: kubeprober/probe-base:v0.0.9
FROM alpine:3.9
WORKDIR /

COPY main.sh .
COPY bin/kubectl /bin
COPY bin/report-status /bin
COPY bin/kubectl-probe-node /usr/local/bin/
RUN chmod 755 /usr/local/bin/kubectl-probe-node
RUN apk add --no-cache jq mysql-client bash glusterfs-cli
RUN mkdir /checker
#USER 65532:65532

CMD [ "sh", "-c", "/main.sh"]