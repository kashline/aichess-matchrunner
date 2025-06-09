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

# Build the Docker image
echo "Building Docker image..."
docker build -t $IMAGE_NAME .

# Run the container with the environment variable
echo "Running Docker container..."
docker run --cpus=2 --rm -e DATABASE_URL -e STOCKFISH_URL -e OPENAI_API_KEY $IMAGE_NAME
