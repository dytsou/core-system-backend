# echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] Deploying Start" >> ./deploy.log

set -e

deploy() {
    local dir=$1
    local use_envsubst=$2
    local enable_error_handling=$3

    mkdir -p "$dir" || true

    envsubst < "./compose.yaml" > "./$dir/compose.yaml" 

    cd "$dir"

    docker compose down
    docker compose pull
    if [ "$enable_error_handling" == "true" ]; then
        docker compose up -d --wait || error_handling
    else
        docker compose up -d --wait
    fi
    
    cd ..
}

error_handling() {
    cd ~
    if [ -d "$VERSION" ]; then
        cd "$VERSION"
        docker logs "$VERSION"
        docker compose down
        cd ..
        rm -r "$VERSION"
    fi
    exit 1
}

if [ "$MODE" == "Snapshot" ] || [ "$MODE" == "Close" ]; then
    export VERSION="pr-$PR_NUMBER"
fi

case "$MODE" in
    "Snapshot")
        flag="false"
        [ ! -d "$VERSION" ] && flag="true"
        deploy "$VERSION" "true" "$flag"

        ;;

    "Close")
        cd /tmp/"$REPO_NAME"-"$VERSION"/repo/.deploy/snapshot/"$VERSION"
        docker compose down
        ;;

    "Dev")
        export VERSION="dev"
        deploy "$VERSION" "true" "false"

        ;;

    "Stage")
        export VERSION="stage"
        deploy "$VERSION" "false" "false"

        ;;
esac

echo "finish"