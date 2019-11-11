FROM python:3-alpine

# Create app directory
COPY . /app
WORKDIR /app

RUN apk update && apk add mysql-client && rm -f /var/cache/apk/* && pip install -r requirements.txt

CMD ./docker-entry.sh
