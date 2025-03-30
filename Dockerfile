FROM alpine:3.21

# Install necessary dependencies for Chromium
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    ttf-freefont

# Set the environment variable for Chromium
ENV CHROME_BIN=/usr/bin/chromium-browser

ARG KIND

COPY ${KIND} app

RUN chmod +x app

CMD ./app