FROM chromedp/headless-shell:138.0.7153.0

RUN apt update && apt install -y ca-certificates

ARG KIND

COPY ${KIND} app

RUN chmod +x app

ENTRYPOINT ["/app"]