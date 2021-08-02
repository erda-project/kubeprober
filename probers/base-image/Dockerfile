## lastet version: kubeprober/probe-base:v0.0.3
FROM centos:7
WORKDIR /

COPY main.sh .
COPY bin/kubectl /bin
COPY bin/report-status /bin
COPY bin/kubectl-probe-node /usr/local/bin/
RUN chmod 755 /usr/local/bin/kubectl-probe-node
RUN mkdir /checker
#USER 65532:65532

CMD [ "sh", "-c", "/main.sh"]