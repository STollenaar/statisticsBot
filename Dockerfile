FROM chromedp/headless-shell:143.0.7499.170

RUN apt update && apt install -y ca-certificates

ARG KIND

COPY ${KIND} app

RUN chmod +x app

ENTRYPOINT ["/app"]