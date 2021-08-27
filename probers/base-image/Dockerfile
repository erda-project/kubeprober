FROM alpine:3.9
WORKDIR /

COPY bin/kubectl /bin
COPY bin/report-status /bin
COPY bin/kubectl-probe* /usr/local/bin/
RUN chmod 755 /usr/local/bin/kubectl-probe*
RUN apk add --no-cache jq mysql-client bash curl bc