#!/usr/bin/env bash

pid=$(netstat -nlp | awk '$4~":8200"{ gsub(/\/.*/,"",$7); print $7 }')

kill $pid

rm run_service.sh
