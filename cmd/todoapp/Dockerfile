FROM golang:1.26-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/cmd/todoapp/exe ./cmd/todoapp

FROM alpine:3.23
WORKDIR /app
RUN apk add --no-cache tzdata
COPY --from=builder /app/cmd/todoapp/exe /app
COPY --from=builder /app/public /app/public
ENV PROJECT_ROOT=/app
CMD ["/app/exe"]
