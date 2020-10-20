#!/bin/bash

set -e

USER=$1

KEYS=$(curl -s https://github.com/${USER}.keys)

set +e
IFS=$'\n' read -rd '' -a KEYS <<<"${KEYS}"
set -e

echo "INSERT OR IGNORE INTO user(name) VALUES(\"${USER}\");" | sqlite3 ./ursa.db

ID=$(echo "SELECT id FROM user WHERE name = \"${USER}\";" | sqlite3 ./ursa.db)

for key in "${KEYS[@]}"
do
    echo "INSERT OR IGNORE INTO key(key, user_id) VALUES(\"${key}\", ${ID});" | sqlite3 ./ursa.db
done
