FROM golang:1.18.0-alpine

ARG ARCH
ARG KIND

# Create app directory
WORKDIR /usr/src/app

COPY ${KIND} app

CMD ./app