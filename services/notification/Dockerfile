FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY *.go ./
ENV GOPROXY=https://goproxy.io,direct
RUN apk add --no-cache git
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o service .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/service .
EXPOSE 8080
CMD ["./service"]
