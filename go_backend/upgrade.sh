#!/bin/bash

# Upgrade script for Alas Cloud Go Backend

echo "🚀 Starting upgrade process..."

# 1. Pull latest code (if in git repo)
# git pull

# 2. Rebuild and restart container
echo "🔄 Rebuilding and restarting container..."
docker compose up -d --build --force-recreate

# 3. Prune old images
echo "🧹 Cleaning up old images..."
docker image prune -f

echo "✅ Upgrade complete!"
