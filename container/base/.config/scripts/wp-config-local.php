<?php
/**
 * Applies the local development constants to wp-config.php.
 *
 * Run by docker-entrypoint.sh as `wp eval-file --skip-wordpress`, so all of
 * them are written in a single wp-cli run rather than one `wp config set`
 * (and one wp-cli boot) each. WPConfigTransformer is what `wp config set`
 * uses itself, bundled with wp-cli.
 */

$config = new WPConfigTransformer( getenv( 'WEBROOT' ) . '/wp-config.php' );

// Raw values are written as PHP code (true, 1), the rest as strings. Empty
// values are skipped rather than written as an invalid define().
$set = function ( $name, $value, $raw = false ) use ( $config ) {
	if ( '' === (string) $value ) {
		return;
	}
	$config->update( 'constant', $name, (string) $value, array( 'raw' => $raw ) );
};

$domain = getenv( 'LOCAL_DOMAIN' );

$set( 'WP_HOME', "https://$domain" );
$set( 'WP_SITEURL', "https://$domain" );
$set( 'WP_DEBUG', getenv( 'WP_DEBUG' ), true );
$set( 'WP_DEBUG_LOG', getenv( 'WP_DEBUG_LOG' ), true );
$set( 'WP_DEBUG_DISPLAY', getenv( 'WP_DEBUG_DISPLAY' ), true );
$set( 'WP_ENVIRONMENT_TYPE', 'local' );
$set( 'WP_DEVELOPMENT_MODE', 'all' );
$set( 'JCORE_IS_LOCAL', 'true', true );
// wp-cron really runs locally (the loopback service routes WordPress's
// requests to its own site), so keep it from auto-updating plugins whose
// auto_update_plugins setting came along with a production database.
$set( 'AUTOMATIC_UPDATER_DISABLED', 'true', true );

// Redis object cache (the SpinupWP drop-in, installed by
// docker-entrypoint.sh). Production on SpinupWP only sets the key salt and
// uses the default connection; locally Redis is its own container.
$set( 'WP_REDIS_HOST', 'redis' );
$set( 'WP_CACHE_KEY_SALT', $domain );

// Left over from the unused memcached container.
if ( $config->exists( 'variable', 'memcached_servers' ) ) {
	$config->remove( 'variable', 'memcached_servers' );
}

if ( 'true' === getenv( 'MULTISITE' ) ) {
	$set( 'WP_ALLOW_MULTISITE', 'true', true );
	$set( 'MULTISITE', 'true', true );
	$set( 'PATH_CURRENT_SITE', '/' );
	$set( 'SUBDOMAIN_INSTALL', 'true', true );
	$set( 'DOMAIN_CURRENT_SITE', $domain );
	$set( 'SITE_ID_CURRENT_SITE', '1', true );
	$set( 'BLOG_ID_CURRENT_SITE', '1', true );
	// Empty on purpose, so it can't go through $set.
	$config->update( 'constant', 'COOKIE_DOMAIN', '' );
}
