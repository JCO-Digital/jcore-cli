if [ -f "$SQLPATH/update.sql" ]; then
  echo "Found update"
  echo "Update table prefix"
  wp config set --path="$WEBROOT" --skip-plugins --skip-themes --type=variable table_prefix $DB_PREFIX

  echo "Import DB"
  wp db import --path="$WEBROOT" --skip-plugins --skip-themes "$SQLPATH/update.sql"
  if [ -f "$SQLPATH/db.sql" ]; then
    mv "$SQLPATH/db.sql" "$SQLPATH/db.old.sql"
  fi
  mv "$SQLPATH/update.sql" "$SQLPATH/db.sql"

  for replaceRecord in $REPLACE; do
    record=(${replaceRecord//|/ })
    if [[ ${#record[*]} -ge 2 ]]; then
      echo "Replace ${record[0]} with ${record[1]}"
      wp search-replace --network --path="$WEBROOT" --skip-plugins --skip-themes "${record[0]}" "${record[1]}"  --recurse-objects --report-changed-only --skip-columns=guid
    fi
  done
fi
