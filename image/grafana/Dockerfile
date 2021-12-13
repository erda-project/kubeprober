FROM grafana/grafana:8.3.1

RUN grafana-cli plugins install grafana-simple-json-datasource
COPY --chown=grafana:root aliyun_cms_grafana_datasource /var/lib/grafana/plugins/aliyun_cms_grafana_datasource