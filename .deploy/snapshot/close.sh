# echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] Deploying Start" >> ./deploy.log

set -e

export VERSION="pr-$PR_NUMBER"

mkdir -p "$VERSION" || true
envsubst < "./compose.yaml" > "./"$VERSION"/compose.yaml"
cd "$VERSION"
docker compose down