FROM chromedp/headless-shell:152.0.7939.3

RUN apt update && apt install -y ca-certificates

ARG KIND

COPY ${KIND} app

RUN chmod +x app

ENTRYPOINT ["/app"]