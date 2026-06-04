FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .
COPY frontend/ frontend/
EXPOSE 8080
CMD ["./server"]