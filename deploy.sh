#!/bin/bash

# Exit immediately if a command fails
set -e
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
else
  echo ".env file not found"
  exit 1
fi

# Define the image name
IMAGE_NAME="aichess-matchrunner"
PROJECT="aichess-457721"

# Build the Docker image
echo "Building Docker image..."
docker buildx build --platform linux/amd64 -t $IMAGE_NAME .

docker tag aichess-matchrunner:latest us-west1-docker.pkg.dev/$PROJECT/$IMAGE_NAME/$IMAGE_NAME:latest

docker push us-west1-docker.pkg.dev/$PROJECT/$IMAGE_NAME/$IMAGE_NAME:latest
