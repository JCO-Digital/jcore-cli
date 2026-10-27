#!/bin/bash

# Stop with a clear error instead of starting php-fpm on a half set-up
# site, which looks like WordPress hanging.
fail() {
  echo "Error: $*" >&2
  echo "Fix the cause, then restart the project (jcore restart)." >&2
  exit 1
}

# Create Private / Public key if it doesn't exist
if [ ! -f ~/.ssh/id_ed25519 ]; then
  ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -P ""
fi

if [[ ! -f "$WEBROOT/index.php" || ! -f "$WEBROOT/wp-includes/version.php" ]]; then
  wp core download --path="$WEBROOT" --version="$WP_VERSION" --skip-content ||
    fail "downloading WordPress $WP_VERSION failed."
fi

# Wait until the DB accepts connections as the WordPress user. On a fresh
# volume (first start, or after `jcore clean`) MariaDB first initialises
# behind a temporary server with networking off, which can take well over
# the few seconds a fixed sleep allows; every step below would then fail.
DB_WAIT_SECONDS=${DB_WAIT_SECONDS:-180}
echo "Waiting for database."
for (( i = 0; i < DB_WAIT_SECONDS; i += 2 )); do
  php -r '
    mysqli_report(MYSQLI_REPORT_OFF);
    [$host, $port] = array_pad(explode(":", getenv("WP_DB_HOST"), 2), 2, 3306);
    $db = @mysqli_connect($host, getenv("WP_DB_USER"), getenv("WP_DB_PASSWORD"), getenv("WP_DB_NAME"), (int) $port);
    exit($db ? 0 : 1);
  ' && break
  sleep 2
done
if (( i >= DB_WAIT_SECONDS )); then
  fail "database not reachable at $WP_DB_HOST after ${DB_WAIT_SECONDS}s."
fi

# Create config file. The database itself is created by the db container
# (MYSQL_DATABASE).
if [[ ! -f "$WEBROOT/wp-config.php" ]]; then
  echo "Creating wp-config.php"
  wp config create --path="$WEBROOT" --dbhost="$WP_DB_HOST" --dbname="$WP_DB_NAME" --dbuser="$WP_DB_USER" --dbpass="$WP_DB_PASSWORD" --dbcharset="$DB_CHARSET" --dbprefix="$DB_PREFIX" ||
    fail "creating wp-config.php failed."
fi

# Add the local development constants (and multisite ones) to
# wp-config.php, all in one wp-cli run. Before anything loads WordPress,
# so the object cache drop-in already finds WP_REDIS_HOST.
wp eval-file /project/.config/scripts/wp-config-local.php --path="$WEBROOT" --skip-wordpress ||
  fail "updating wp-config.php failed."

# Redis object cache, as on production: SpinupWP's plugin installs its own
# object-cache.php drop-in there, but only on SpinupWP servers, so copy it
# in from the project's copy of the plugin. Skipped when the objectCache
# setting is off, or for Object Cache Pro, which the plugin skips too.
# The drop-in needs the PhpRedis extension: without it, it falls back to
# Predis, which the plugin doesn't ship, and every request dies, so an
# image older than PhpRedis gets no drop-in.
# Only a drop-in that came from SpinupWP is ever replaced or removed.
SPINUPWP_DROPIN="$WEBROOT/wp-content/plugins/spinupwp/drop-ins/object-cache.php"
OBJECT_CACHE_FILE="$WEBROOT/wp-content/object-cache.php"
OBJECT_CACHE_MARKER="SpinupWP Redis Object Cache Drop-In"
HAS_PHPREDIS=true
php -r 'exit(class_exists("Redis") ? 0 : 1);' || HAS_PHPREDIS=false
if [[ $HAS_PHPREDIS == false && -f $SPINUPWP_DROPIN && $OBJECT_CACHE != false ]]; then
  echo "This image has no PhpRedis, so the Redis object cache is off. Update the image (jcore start --install pulls it)."
fi
if [[ $OBJECT_CACHE != false && $HAS_PHPREDIS == true && -f $SPINUPWP_DROPIN && ! -d $WEBROOT/wp-content/plugins/object-cache-pro ]]; then
  if [[ -f $OBJECT_CACHE_FILE ]] && ! grep -q "$OBJECT_CACHE_MARKER" "$OBJECT_CACHE_FILE"; then
    echo "Keeping the existing (non-SpinupWP) object-cache.php."
  elif ! cmp -s "$SPINUPWP_DROPIN" "$OBJECT_CACHE_FILE"; then
    echo "Installing the SpinupWP Redis object cache drop-in."
    cp "$SPINUPWP_DROPIN" "$OBJECT_CACHE_FILE" || fail "installing object-cache.php failed."
  fi
elif grep -qs "$OBJECT_CACHE_MARKER" "$OBJECT_CACHE_FILE"; then
  echo "Removing the SpinupWP object cache drop-in (objectCache is off, no PhpRedis, or the plugin is gone)."
  rm -f "$OBJECT_CACHE_FILE"
fi

# Install WP.
if wp core is-installed --path="$WEBROOT"; then
  echo "WP is already installed."
else
  echo "WP not installed, installing."
  wp core install --path="$WEBROOT" --title="WordPress" --url="https://$LOCAL_DOMAIN" --admin_user="$WP_ADMIN_USER" --admin_email="$WP_ADMIN_EMAIL" --admin_password="$WP_ADMIN_PASSWORD" ||
    fail "installing WordPress failed."
fi

/project/.config/scripts/appendssl

# Start PHP-FPM. exec, so it replaces this script and receives the stop
# signal from `docker stop` directly; otherwise the container ignores it
# and Docker waits its full timeout before killing it.
exec php-fpm
