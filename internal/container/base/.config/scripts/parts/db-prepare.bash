mkdir -p $SQLPATH

if [ -f $SQLPATH/update.sql.gz ]; then
  gunzip $SQLPATH/update.sql.gz
fi
