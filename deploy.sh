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
# IMAGE_NAME="aichess-matchrunner"
# PROJECT="aichess-457721"

AWS_REGION="us-west-2"
ACCOUNT_ID="079152373621"
REPO_NAME="aichess-matchrunner"
IMAGE_TAG="latest"

IMAGE_URI="${ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${REPO_NAME}:${IMAGE_TAG}"

aws ecr get-login-password --region $AWS_REGION | \
  docker login --username AWS --password-stdin "${ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"


# Build the Docker image
echo "Building Docker image..."
docker buildx build --platform linux/amd64 -t $IMAGE_URI .

# docker tag aichess-matchrunner:latest us-west1-docker.pkg.dev/$PROJECT/$IMAGE_NAME/$IMAGE_NAME:latest

docker push "$IMAGE_URI"
