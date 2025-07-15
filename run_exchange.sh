#!/bin/bash

# Load and tag images (using full paths)
echo "Loading Docker images..."
docker load -i ./exchange/exchange1_amd64.tar
docker load -i ./exchange/exchange2_amd64.tar
docker load -i ./exchange/exchange3_amd64.tar


echo "Images loaded and tagged."