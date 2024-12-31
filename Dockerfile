FROM alpine:3.21

ARG KIND

COPY ${KIND} app

RUN chmod +x app

CMD ./app