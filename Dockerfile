FROM alpine:3.21

WORKDIR /usr/src/app

COPY ${KIND} app

RUN chmod +x app

CMD ./app