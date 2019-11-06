USER=stats
USER_PASS=$USER

service mysql start

mysql -u root --password="$USER_PASS" -h localhost <<-EOSQL
 CREATE DATABASE IF NOT EXISTS STATS_DB;
 GRANT ALL ON STATS_DB.* TO '$USER' IDENTIFIED BY '$USER_PASS';
EOSQL


cat ./stats.sql | mysql -u "$USER" --password="$USER_PASS" STATS_DB