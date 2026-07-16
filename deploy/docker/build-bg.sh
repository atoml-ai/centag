#!/bin/bash
cd /vol1/1000/Data/devspaces/centag
docker build -t centag:latest -f deploy/docker/Dockerfile . > /tmp/docker_build.log 2>&1
