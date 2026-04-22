FROM golang:1.24 AS builder

WORKDIR /src
COPY rare-anomaly-tester.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -buildvcs=false -o rare-anomaly-tester rare-anomaly-tester.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /src/rare-anomaly-tester /app/rare-anomaly-tester

ENTRYPOINT ["/app/rare-anomaly-tester"]