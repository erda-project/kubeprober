FROM kubeprober/probe-base:v0.0.2
WORKDIR /

COPY checker/ /checker/
#USER 65532:65532

CMD [ "sh", "-c", "/main.sh"]