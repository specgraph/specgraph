FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /specgraph ./cmd/specgraph

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /specgraph /usr/local/bin/specgraph
EXPOSE 9090
ENTRYPOINT ["specgraph"]
CMD ["serve", "--config", "/etc/specgraph/config.yaml"]
