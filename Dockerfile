FROM golang:1.23.4

ARG ARCH
ARG KIND

# Create app directory
WORKDIR /usr/src/app

COPY ${KIND} app

CMD ./app