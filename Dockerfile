FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o glance .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/glance /app/glance
RUN chmod +x /app/glance

EXPOSE 8080/tcp

ENTRYPOINT ["/app/glance"]