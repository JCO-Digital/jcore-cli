#!/bin/bash

# Ensure wp-cli is present and current. The base image bundles its own
# wp-cli binary, which is easy to forget to bump when rebuilding the image,
# so take ownership of it here and always bring it up to date instead of
# relying on the image's baked-in version.
WP_CLI_BIN="/usr/local/bin/wp"
if [ ! -f "$WP_CLI_BIN" ]; then
  echo "wp-cli not found, installing."
  curl -sSL -o /tmp/wp-cli.phar https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar
  chmod +x /tmp/wp-cli.phar
  sudo install -o "$(id -u)" -g "$(id -g)" /tmp/wp-cli.phar "$WP_CLI_BIN"
  rm -f /tmp/wp-cli.phar
else
  sudo chown "$(id -u):$(id -g)" "$WP_CLI_BIN"

  # Only check for updates once a day to avoid hitting GitHub on every start.
  WP_CLI_UPDATE_STAMP="$HOME/.wp-cli-update-stamp"
  LAST_UPDATE=0
  [ -f "$WP_CLI_UPDATE_STAMP" ] && LAST_UPDATE=$(cat "$WP_CLI_UPDATE_STAMP")
  if (( $(date +%s) - LAST_UPDATE > 86400 )); then
    echo "Updating wp-cli."
    wp cli update --stable --yes
    date +%s > "$WP_CLI_UPDATE_STAMP"
  fi
fi

# Create Private / Public key if it doesn't exist
if [ ! -f ~/.ssh/id_ed25519 ]; then
  ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -P ""
fi

if [[ ! -f "$WEBROOT/index.php" || ! -f "$WEBROOT/wp-includes/version.php" ]]; then
  wp core download --path="$WEBROOT" --version="$WP_VERSION" --skip-content
fi

# Wait until the DB accepts connections as the WordPress user. On a fresh
# volume (first start, or after `jcore clean`) MariaDB first initialises
# behind a temporary server with networking off, which can take well over
# the few seconds a fixed sleep allows; every step below would then fail.
DB_WAIT_SECONDS=180
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
  echo "Error: database not reachable at $WP_DB_HOST after ${DB_WAIT_SECONDS}s; WordPress setup will likely fail."
fi

# Create config file.
if [[ ! -f "$WEBROOT/wp-config.php" ]]; then
  echo "Creating wp-config.php"
  wp config create --path="$WEBROOT" --dbhost="$WP_DB_HOST" --dbname="$WP_DB_NAME" --dbuser="$WP_DB_USER" --dbpass="$WP_DB_PASSWORD" --dbcharset="$DB_CHARSET" --dbprefix="$DB_PREFIX"
  wp config set --path="$WEBROOT" memcached_servers "array( array( 'memcached', 11211 ) )" --raw --type=variable
  echo "Creating DB."
  wp db create --path="$WEBROOT"
fi

# Install WP.
if wp core is-installed --path="$WEBROOT"; then
  echo "WP is already installed."
else
  echo "WP not installed, installing."
  wp core install --path="$WEBROOT" --title="WordPress" --url="$DOMAIN" --admin_user="$WP_ADMIN_USER" --admin_email="$WP_ADMIN_EMAIL" --admin_password="$WP_ADMIN_PASSWORD"
fi

# Add constants to wp-config.php
wp config set --path="$WEBROOT" --type=constant WP_HOME "https://$LOCAL_DOMAIN"
wp config set --path="$WEBROOT" --type=constant WP_SITEURL "https://$LOCAL_DOMAIN"
wp config set --path="$WEBROOT" --type=constant WP_DEBUG $WP_DEBUG --raw
wp config set --path="$WEBROOT" --type=constant WP_DEBUG_LOG $WP_DEBUG_LOG --raw
wp config set --path="$WEBROOT" --type=constant WP_DEBUG_DISPLAY $WP_DEBUG_DISPLAY --raw
wp config set --path="$WEBROOT" --type=constant WP_ENVIRONMENT_TYPE "local"
wp config set --path="$WEBROOT" --type=constant WP_DEVELOPMENT_MODE "all"
wp config set --path="$WEBROOT" --type=constant JCORE_IS_LOCAL true --raw
# wp-cron really runs locally (the loopback service routes WordPress's
# requests to its own site), so keep it from auto-updating plugins whose
# auto_update_plugins setting came along with a production database.
wp config set --path="$WEBROOT" --type=constant AUTOMATIC_UPDATER_DISABLED true --raw

# Add multisite configuration constants
if [[ $MULTISITE == true ]]; then
	wp config set --path="$WEBROOT" --type=constant WP_ALLOW_MULTISITE true --raw
	wp config set --path="$WEBROOT" --type=constant MULTISITE true --raw
	wp config set --path="$WEBROOT" --type=constant PATH_CURRENT_SITE "/"
	wp config set --path="$WEBROOT" --type=constant SUBDOMAIN_INSTALL true --raw
	wp config set --path="$WEBROOT" --type=constant DOMAIN_CURRENT_SITE "$LOCAL_DOMAIN"
	wp config set --path="$WEBROOT" --type=constant SITE_ID_CURRENT_SITE 1 --raw
	wp config set --path="$WEBROOT" --type=constant BLOG_ID_CURRENT_SITE 1 --raw
	wp config set --path="$WEBROOT" --type=constant COOKIE_DOMAIN ""
fi

/project/.config/scripts/appendssl

# Start PHP-FPM
php-fpm
