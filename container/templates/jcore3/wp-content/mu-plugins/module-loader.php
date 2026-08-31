<?php
/**
 * JCore Module Loader
 */

if ( ! defined( 'ABSPATH' ) ) {
	exit();
}

if ( is_readable( trailingslashit( ABSPATH ) . 'vendor/autoload.php' ) ) {
	require_once trailingslashit( ABSPATH ) . 'vendor/autoload.php';
}
