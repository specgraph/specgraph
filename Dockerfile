# syntax=docker/dockerfile:1
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN --mount=type=secret,id=extra-certs,target=/tmp/extra-ca-bundle.pem,required=false \
    if [ -s /tmp/extra-ca-bundle.pem ]; then cat /tmp/extra-ca-bundle.pem >> /etc/ssl/certs/ca-certificates.crt; fi && \
    apk upgrade --no-cache && apk add --no-cache ca-certificates && \
    if [ -s /tmp/extra-ca-bundle.pem ]; then cat /tmp/extra-ca-bundle.pem >> /etc/ssl/certs/ca-certificates.crt; fi
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/specgraph /usr/local/bin/specgraph
EXPOSE 9090
RUN addgroup -S specgraph && adduser -S specgraph -G specgraph -h /home/specgraph
ENV HOME=/home/specgraph
USER specgraph
ENTRYPOINT ["specgraph"]
CMD ["serve", "--config", "/etc/specgraph/config.yaml"]
