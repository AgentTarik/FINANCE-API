# build
FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o finance-api ./cmd

# runtime
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/finance-api /app/finance-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/finance-api"]
