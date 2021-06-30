FROM golang:1.16.0-alpine

# Create app directory
WORKDIR /usr/src/app

COPY . .

RUN go get .

CMD go run .
