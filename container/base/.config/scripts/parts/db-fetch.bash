if [ ! -f $SQLPATH/update.sql ]; then
  exclude=""
  # Exclude tables from export
  if [ -n "$DB_EXCLUDE" ]; then
    ARGS="--exclude_tables="
    for table in $DB_EXCLUDE ; do
      ARGS+="${DB_PREFIX}${table},"
    done
    exclude=`echo ${ARGS} | sed s/,$//`
  fi
  mkdir -p sql
  echo "Fetching DB from $REMOTE_HOST"
  wp db export ${exclude} --single-transaction --quick --lock-tables=false --skip-plugins --skip-themes --ssh="$REMOTE_HOST" --path="$REMOTE_PATH" - > "$SQLPATH/update.sql" || rm "$SQLPATH/update.sql"
fi
