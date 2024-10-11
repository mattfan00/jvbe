# syntax=docker/dockerfile:1 
FROM golang:1.21-alpine as app

RUN apk add gcc musl-dev # install gcc needed for cgo

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o jvbe ./cmd/jvbe


FROM alpine:3.19

WORKDIR /app

COPY --from=app /app/jvbe ./

ENTRYPOINT [ "./jvbe" ]
