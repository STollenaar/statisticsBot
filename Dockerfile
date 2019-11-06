FROM python:3

# Create app directory
COPY . /app
WORKDIR /app

RUN apt-get update && \
 apt-get install -y build-essential && \
 apt-get install -y mariadb-server && \
 apt-get install -y mariadb-client && \ 
 pip install -r requirements.txt

RUN ./init-db.sh

CMD ./docker-entry.sh
