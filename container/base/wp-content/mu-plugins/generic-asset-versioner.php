<?php
/**
 * Plugin Name: Generic Asset Versioner
 * Description: Automatically appends file modification time as the version query string for enqueued local scripts and styles.
 * Version:     0.1.0
 * Author:      J&Co Digital
 *
 * @package Jcore\AssetVersioner
 */

declare(strict_types=1);

namespace Jcore\AssetVersioner;

if ( ! defined( 'ABSPATH' ) ) {
	exit;
}

/**
 * Filter script source URL to append/update the version based on file modification time.
 *
 * @param string $src    Script source URL.
 * @param string $handle Script handle.
 * @return string Filtered script URL.
 */
function filter_script_loader_src( string $src, string $handle ): string {
	return version_asset_url( $src );
}
add_filter( 'script_loader_src', __NAMESPACE__ . '\\filter_script_loader_src', 20, 2 );

/**
 * Filter style source URL to append/update the version based on file modification time.
 *
 * @param string $src    Style source URL.
 * @param string $handle Style handle.
 * @return string Filtered style URL.
 */
function filter_style_loader_src( string $src, string $handle ): string {
	return version_asset_url( $src );
}
add_filter( 'style_loader_src', __NAMESPACE__ . '\\filter_style_loader_src', 20, 2 );

/**
 * Modifies an asset URL to use the file modification time as ?ver=.
 *
 * @param string $src Asset URL.
 * @return string Asset URL with updated version, or original URL if not a local file.
 */
function version_asset_url( string $src ): string {
	if ( empty( $src ) ) {
		return $src;
	}

	$filepath = resolve_local_filepath( $src );
	if ( ! $filepath ) {
		return $src;
	}

	$mtime = get_file_mtime( $filepath );
	if ( false === $mtime ) {
		return $src;
	}

	return add_query_arg( 'ver', (string) $mtime, $src );
}

/**
 * In-memory cached filemtime lookup to avoid redundant filesystem calls during a single request.
 *
 * @param string $filepath Absolute file path.
 * @return int|false Timestamp on success, false if file does not exist or cannot be read.
 */
function get_file_mtime( string $filepath ): int|false {
	static $cache = array();

	if ( array_key_exists( $filepath, $cache ) ) {
		return $cache[ $filepath ];
	}

	// file_exists and filemtime share the PHP stat cache.
	if ( ! file_exists( $filepath ) ) {
		$cache[ $filepath ] = false;
		return false;
	}

	$mtime              = filemtime( $filepath );
	$cache[ $filepath ] = $mtime;

	return $mtime;
}

/**
 * Resolves an asset URL or relative path to an absolute filesystem path.
 * Handles content URL, site URL, wp-includes, and root-relative URLs.
 *
 * @param string $src Asset URL.
 * @return string|null Absolute filesystem path, or null if external or cannot be resolved.
 */
function resolve_local_filepath( string $src ): ?string {
	// Strip query parameters and fragment to get just the path.
	$url_path = wp_parse_url( $src, PHP_URL_PATH );
	if ( empty( $url_path ) ) {
		return null;
	}

	// 1. Check for external hosts.
	$src_host = wp_parse_url( $src, PHP_URL_HOST );
	if ( ! empty( $src_host ) ) {
		$site_host    = wp_parse_url( site_url(), PHP_URL_HOST );
		$content_host = wp_parse_url( content_url(), PHP_URL_HOST );

		// If host is specified and does not match site_url or content_url host, it's external.
		if ( $src_host !== $site_host && $src_host !== $content_host ) {
			return null;
		}
	}

	// 2. Try matching against WP_CONTENT_URL -> WP_CONTENT_DIR (most common for themes & plugins).
	$content_url_path = wp_parse_url( content_url(), PHP_URL_PATH );
	if ( ! empty( $content_url_path ) && str_starts_with( $url_path, $content_url_path ) ) {
		$relative = substr( $url_path, strlen( $content_url_path ) );
		$path     = wp_normalize_path( WP_CONTENT_DIR . '/' . ltrim( $relative, '/' ) );
		return file_exists( $path ) ? $path : null;
	}

	// 3. Fallback to site_url() / ABSPATH for core assets (e.g. wp-includes, wp-admin).
	$site_url_path = wp_parse_url( site_url(), PHP_URL_PATH );
	if ( ! empty( $site_url_path ) && '/' !== $site_url_path && str_starts_with( $url_path, $site_url_path ) ) {
		$relative = substr( $url_path, strlen( $site_url_path ) );
		$path     = wp_normalize_path( ABSPATH . '/' . ltrim( $relative, '/' ) );
		return file_exists( $path ) ? $path : null;
	}

	// 4. Fallback for root-relative paths starting directly from ABSPATH.
	$path = wp_normalize_path( ABSPATH . '/' . ltrim( $url_path, '/' ) );
	if ( file_exists( $path ) ) {
		return $path;
	}

	return null;
}
