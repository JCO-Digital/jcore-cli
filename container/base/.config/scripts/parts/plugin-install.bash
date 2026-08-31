for PLUGIN in $PLUGIN_LOCAL ; do
  if wp plugin is-installed --path="$WEBROOT" "$PLUGIN"
  then
      wp plugin activate --path="$WEBROOT" "$PLUGIN"
  else
    wp plugin install --path="$WEBROOT" --activate "$PLUGIN"
  fi
done
