FROM chromedp/headless-shell:159.0.8130.0

RUN apt update && apt install -y ca-certificates

ARG KIND

COPY ${KIND} app

RUN chmod +x app

ENTRYPOINT ["/app"]