FROM chromedp/headless-shell:134.0.6998.178

RUN apt update && apt install -y ca-certificates

ARG KIND

COPY ${KIND} app

RUN chmod +x app

ENTRYPOINT ["/app"]