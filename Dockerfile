FROM python:alpine

# Create app directory
COPY . /app
WORKDIR /app

RUN apk add --no-cache --virtual .build-deps gcc musl-dev \
  .build-deps && apk update && apk add mysql-client && pip install -r requirements.txt && apk del

CMD ./docker-entry.sh
