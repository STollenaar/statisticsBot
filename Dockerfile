FROM golang:1.18.0-alpine

# Create app directory
WORKDIR /usr/src/app

COPY . .

RUN go get .

CMD go run .
