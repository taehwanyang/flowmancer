FROM golang:1.24 AS builder

WORKDIR /src
COPY volume-anomaly-tester.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -buildvcs=false -o volume-anomaly-tester volume-anomaly-tester.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /src/volume-anomaly-tester /app/volume-anomaly-tester

ENTRYPOINT ["/app/volume-anomaly-tester"]