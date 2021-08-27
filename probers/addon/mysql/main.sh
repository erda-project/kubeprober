#!/bin/bash
cd "$(dirname "${BASH_SOURCE[0]}")"

MYSQL_HOST=$(kubectl  get cm dice-addons-info -n default -o jsonpath='{.data.MYSQL_HOST}')
MYSQL_USERNAME=$(kubectl  get cm dice-addons-info -n default -o jsonpath='{.data.MYSQL_USERNAME}')
MYSQL_PORT=$(kubectl  get cm dice-addons-info -n default -o jsonpath='{.data.MYSQL_PORT}')
MYSQL_DATABASE=$(kubectl  get cm dice-addons-info -n default -o jsonpath='{.data.MYSQL_DATABASE}')
MYSQL_PASSWORD=$(kubectl  get cm dice-addons-info -n default -o jsonpath='{.data.MYSQL_PASSWORD}')

function check_mysql() {
  #check mysql connected
  mysql -h $MYSQL_HOST -u $MYSQL_USERNAME  -P $MYSQL_PORT -p$MYSQL_PASSWORD -e "select now()" >/dev/null 2>/dev/null
  if [[ $? != 0 ]]; then
    report-status --name=check_mysql --status=error --message="mysql not connected"
    return 1
  fi

  #check mysql connection percent
  max_connections=$(mysql -h $MYSQL_HOST -u $MYSQL_USERNAME  -P $MYSQL_PORT -p$MYSQL_PASSWORD -e "show variables like '%max_connections%';" | grep max_connections | awk '{print $2}')
  threads_connected=$(mysql -h $MYSQL_HOST -u $MYSQL_USERNAME  -P $MYSQL_PORT -p$MYSQL_PASSWORD -e " show status like 'Threads%';;" | grep Threads_connected | awk '{print $2}')
  connection_percent=$(echo "scale=2; $threads_connected / $max_connections * 100" | bc | awk -F"." '{print $1}')
  if [[ "$connection_percent" -gt 85 ]]; then
    report-status --name=check_mysql --status=error  --message="too many connections: $connection_percent%"
    return 1
  fi

  #check mysql create table && insert data
  msg=$(mysql -h $MYSQL_HOST -u $MYSQL_USERNAME  -P $MYSQL_PORT -p$MYSQL_PASSWORD $MYSQL_DATABASE -e "CREATE TABLE IF NOT EXISTS probe_test_table (id INT(11), name VARCHAR(25))" 2>&1)
  if [[ $msg != "" ]]; then
    report-status --name=check_mysql --status=error --message="could not create table: $msg"
    return 1
  fi

  msg=$(mysql -h $MYSQL_HOST -u $MYSQL_USERNAME  -P $MYSQL_PORT -p$MYSQL_PASSWORD $MYSQL_DATABASE -e "INSERT INTO probe_test_table (id, name) VALUES (1, 'prober')" 2>&1)
  if [[ $? != 0 ]]; then
    report-status --name=check_mysql --status=error --message="could not insert data to table: $msg"
    return 1
  fi

  msg=$(mysql -h $MYSQL_HOST -u $MYSQL_USERNAME  -P $MYSQL_PORT -p$MYSQL_PASSWORD $MYSQL_DATABASE -e "DROP TABLE IF EXISTS probe_test_table" 2>&1)
  if [[ $msg != "" ]]; then
    report-status --name=check_mysql --status=error --message="could not drop table: $msg"
    return 1
  fi

  report-status --name=check_mysql --status=pass --message="-"
}

if kubectl get cm dice-cluster-info -n default -o yaml | grep DICE_IS_EDGE: | grep false>/dev/null 2>/dev/null; then
  check_mysql
fi