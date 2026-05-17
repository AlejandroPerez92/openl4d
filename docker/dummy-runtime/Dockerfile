FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY main.go /src/main.go
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/dummy-runtime /src/main.go

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/dummy-runtime /usr/local/bin/dummy-runtime
ENTRYPOINT ["/usr/local/bin/dummy-runtime"]
