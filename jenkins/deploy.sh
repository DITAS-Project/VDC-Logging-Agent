#!/usr/bin/env bash
# Staging environment: 31.171.247.162
# Private key for ssh: /opt/keypairs/ditas-testbed-keypair.pem

# TODO state management? We are killing without careing about any operation the conainer could be doing.

ssh -i /opt/keypairs/ditas-testbed-keypair.pem cloudsigma@31.171.247.162 << 'ENDSSH'
sudo docker stop --time 20 vdc-logging-agent || true
sudo docker rm --force vdc-logging-agent || true
sudo docker pull ditas/IMAGE_NAME:v02

# Get the host IP
HOST_IP="$(ip route get 8.8.8.8 | awk '{print $NF; exit}')"

# Run the docker mapping the ports and passing the host IP via the environmental variable "DOCKER_HOST_IP"
sudo docker run -p 58484:8484 -e DOCKER_HOST_IP=$HOST_IP --restart unless-stopped -d --name vdc-logging-agent ditas/vdc-logging-agent:v02
ENDSSH
