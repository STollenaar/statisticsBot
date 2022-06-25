FROM golang:1.18.0-alpine

ARG ARCH


# Create app directory
WORKDIR /usr/src/app

COPY statisticsBot .

CMD ./statisticsBot