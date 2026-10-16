# syntax=docker/dockerfile:1
FROM golang:1.27

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /product-service

EXPOSE 8080

CMD ["/product-service"]
