FROM golang:1.26-alpine

WORKDIR /app

# Установка bash
RUN apk add --no-cache bash

COPY go.mod go.sum ./
RUN go mod download

COPY . .

COPY wait-for-it.sh .
COPY start-tracker.sh .

RUN chmod +x wait-for-it.sh start-tracker.sh

RUN go build -o tracker main.go

EXPOSE 9000

CMD ["./start-tracker.sh"]