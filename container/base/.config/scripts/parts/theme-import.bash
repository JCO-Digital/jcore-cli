echo "Syncing themes"
rsync -r --delete --info=progress2 --no-inc-recursive "${REMOTE_HOST}:${REMOTE_PATH}/wp-content/themes/" "$WEBROOT/wp-content/themes/" --exclude=twenty*
