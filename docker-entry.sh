chmod -R ug+rw /var/lib/mysql
chown -R mysql:mysql /var/lib/mysql

service mysql start


# ./init-db.sh

python bot.py
