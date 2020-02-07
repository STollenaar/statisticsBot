FROM python:alpine

# Create app directory
COPY ./requirements.txt /app/requirements.txt
WORKDIR /app

RUN apk add --virtual .build-deps gcc musl-dev \
  .build-deps && apk update && apk add mysql-client && pip install -r requirements.txt && apk del

COPY . /app
CMD ./docker-entry.sh
