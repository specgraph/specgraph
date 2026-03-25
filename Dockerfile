FROM alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659
RUN apk add --no-cache ca-certificates
COPY specgraph /usr/local/bin/specgraph
EXPOSE 9090
RUN addgroup -S specgraph && adduser -S specgraph -G specgraph -h /home/specgraph
ENV HOME=/home/specgraph
USER specgraph
ENTRYPOINT ["specgraph"]
CMD ["serve", "--config", "/etc/specgraph/config.yaml"]
