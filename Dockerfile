FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o virena

FROM alpine:latest

COPY --from=builder /app/virena /virena

EXPOSE 8080

CMD ["/virena"]
