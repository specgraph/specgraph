FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY specgraph /usr/local/bin/specgraph
EXPOSE 9090
ENTRYPOINT ["specgraph"]
CMD ["serve", "--config", "/etc/specgraph/config.yaml"]
