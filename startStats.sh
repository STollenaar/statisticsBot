#git pull

SCRIPT_DIR=$(cd $(dirname "$0"); pwd)

docker build --rm -t statisticsbot . 

docker run --name statisticsbot -t -i --log-driver=journald -v statisticsBot:/var/ statisticsbot

