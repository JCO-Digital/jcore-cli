#!/bin/bash

# Create Private / Public key if it doesn't exist
if [ ! -f ~/.ssh/id_ed25519 ]; then
  ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -P ""
fi

if [[ ! -f "$WEBROOT/index.php" || ! -f "$WEBROOT/wp-includes/version.php" ]]; then
  wp core download --path="$WEBROOT" --version="$WP_VERSION" --skip-content
fi

# Create config file.
if [[ ! -f "$WEBROOT/wp-config.php" ]]; then
  echo "Waiting to configure in case DB is not created."
  sleep 8
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
