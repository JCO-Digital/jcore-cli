# WordPress Container

A robust, containerized WordPress development environment by JCO Digital. This setup provides a complete stack for local WordPress development, including automated installation, SSL termination, database management, and mail trapping.

## Features

- **Automated Setup**: Automatic WordPress core download and installation.
- **SSL/TLS**: Built-in certificate generation for local HTTPS.
- **Database Management**: MariaDB with Adminer for easy database access.
- **Mail Trapping**: Mailpit to catch all outgoing emails during development.
- **Object Cache**: Redis with SpinupWP's object cache drop-in, as on production.
- **Tooling**: Includes WP-CLI, Composer, and SSH agent forwarding.
- **Custom Scripts**: Pre-configured scripts for importing databases, media, and plugins.

## Prerequisites

Ensure you have the following installed on your host machine:

- Docker and Docker Compose
- Node.js & npm / pnpm (for linting and local tools)
- Composer

## Getting Started

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd wordpress-container
   ```

2. **Configure Environment**:
   Create a `.env` file or modify `env-values.toml` to set your local domain and database credentials.

3. **Start the containers**:
   ```bash
   jcore start
   ```
   On the first run, the `tlscerts` container will generate local certificates, and the `wordpress` container will download WordPress core and set up the database.

4. **Access the site**:
   Once the containers are running, navigate to the `LOCAL_DOMAIN` you configured (defaulting to the Nginx service aliases).

## Service Overview

Several projects can run at the same time: a shared JCore proxy (Traefik,
started automatically by `jcore start`) owns host ports `80`/`443` and routes
each request to the right project by its domain.

- **Web (Nginx)**: Serves `https://LOCAL_DOMAIN` (and `*.LOCAL_DOMAIN` for multisite) through the proxy, with its own TLS certificate.
- **WordPress (PHP-FPM)**: The main PHP container (running as user 1000).
- **Database (MariaDB)**: Persistent database storage.
- **Adminer**: Database management UI at `http://adminer.LOCAL_DOMAIN`.
- **Mailhog (Mailpit)**: Email testing tool. UI at `http://mail.LOCAL_DOMAIN`.
- **Redis**: Persistent object cache. Projects with the SpinupWP plugin get its `object-cache.php` drop-in, as on production (turn off with `objectCache = false`). Flushed after every database import; after editing the database directly, flush it with `jcore run "wp cache flush"`.
- **Loopback**: Forwards WordPress's requests to its own site (wp-cron, REST, multisite subsites) to Nginx.

## Configuration

### Environment Variables
The environment is configured via `.env` (generated) and `env-values.toml`. Key variables include:

- `LOCAL_DOMAIN`: The local development URL (e.g., `project.test`).
- `WP_DB_NAME`, `WP_DB_USER`, `WP_DB_PASSWORD`: Database credentials (default: `wordpress`).
- `WP_IMAGE`: The Docker image for the WordPress service.
- `MULTISITE`: Set to `true` to enable WordPress Multisite (Subdomain install).
- `WP_DEBUG`: Boolean to toggle WordPress debug mode.

### Compose Files
jcore generates the project's compose file, `compose.jcore.yml`, on every
start, so every project runs on the current setup. Don't edit or commit it
(it's in `.gitignore`).

Put project-specific changes (extra services, ports, volumes) in
`compose.override.yml`, which jcore merges on top of the generated file.
Commit that one.

Older projects carry their own `docker-compose.yml`. Without local changes
jcore ignores it and it can be deleted. With local changes jcore keeps using
it, but updating the project no longer updates it: move the project's own
changes into `compose.override.yml`, then delete `docker-compose.yml`.

### PHP Configuration
Custom PHP settings can be adjusted in `php.ini`. The default configuration is optimized for development:
- `memory_limit`: 256M
- `upload_max_filesize`: 50M
- `post_max_size`: 75M
- **Xdebug**: Pre-configured to connect to `host.docker.internal` in `develop` mode.

### File Structure
- `wp-content/`: This directory is mapped to the container and contains your themes, plugins, and uploads.
- `.config/`: Contains Nginx configurations and entrypoint scripts.
- `.jcore/`: Internal storage for WordPress core files, SSL certificates, and SQL dumps.

## Pulling From Production

`jcore pull` fetches the database, plugins, themes and media from the remote
server (see `jcore pull --help`).

## Development Tools

### Linting and Formatting
The project uses WordPress coding standards and Prettier.
```bash
pnpm install
# Run prettier
pnpm exec prettier --write .
```

### WP-CLI
You can run WP-CLI commands directly through the wordpress container:
```bash
jcore run "wp plugin list"
```

## Templates

The `templates/` directory contains various JCORE theme configurations defined in `templates.toml`. These templates facilitate scaffolding new projects with specific theme versions or child theme structures:

- **jcore3**: Uses the "hurricane" branch of the `jcore-ilme` theme.
- **jcore2**: Sets up a child theme (branches like "grasshopper") and includes `jcore2` as a git submodule.
- **jcore1**: Sets up a child theme with the original `jcore` theme as a submodule.
- **blank**: A clean starting point for custom development.

### Scaffolding a Template
Template settings are applied during the initial container setup or can be manually triggered if the project configuration supports it. Submodules are automatically mapped to `wp-content/themes/`.

## License
This project is licensed under the GPL.
