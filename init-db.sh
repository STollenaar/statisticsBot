USER=stats
USER_PASS=$USER

mysql -u root --password=spices -h databases <<-EOSQL
 CREATE DATABASE IF NOT EXISTS STATS_DB;
 GRANT ALL ON STATS_DB.* TO '$USER' IDENTIFIED BY '$USER_PASS';
EOSQL


mysql -u $USER --password=$USER_PASS -h databases STATS_DB < stats.sql