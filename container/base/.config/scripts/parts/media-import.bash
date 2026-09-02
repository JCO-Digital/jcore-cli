echo "Syncing media"
rsync -r --delete --info=progress2 --no-inc-recursive "${REMOTE_HOST}:${REMOTE_PATH}/wp-content/uploads/" "$WEBROOT/wp-content/uploads/" --exclude=wp-migrate-db
