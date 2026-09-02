ARGS=()
# shellcheck disable=SC2068
for e in ${PLUGIN_EXCLUDE} ${PLUGIN_GIT} ; do
  echo "$e"
  ARGS+=(--exclude=$e)
done

echo "Syncing plugins"
# shellcheck disable=SC2068
rsync -r --delete --info=progress2 --no-inc-recursive "${REMOTE_HOST}:${REMOTE_PATH}/wp-content/plugins/" "${WEBROOT}/wp-content/plugins/" --exclude=.gitignore ${ARGS[@]}
echo "Plugins synced"
