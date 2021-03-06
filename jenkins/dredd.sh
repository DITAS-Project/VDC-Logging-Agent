#!/usr/bin/env bash

COMPONENT_URL="http://31.171.247.162:58484"
YAML_FILE="api/openapi.v1.yml"

attempts=0
until $(curl --output /dev/null --silent --head --fail $COMPONENT_URL); do
    if [ ${attempts} -eq 20 ];then
      echo "Max attempts reached"
      exit 1
    fi

    printf '.'
    attempts=$(($attempts+1))
    sleep 9
done

dredd $YAML_FILE $COMPONENT_URL
