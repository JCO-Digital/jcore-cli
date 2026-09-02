# WordPress Container

A robust, containerized WordPress development environment by JCO Digital. This setup provides a complete stack for local WordPress development, including automated installation, SSL termination, database management, and mail trapping.

## Features

- **Automated Setup**: Automatic WordPress core download and installation.
- **SSL/TLS**: Built-in certificate generation for local HTTPS.
- **Database Management**: MariaDB with Adminer for easy database access.
- **Mail Trapping**: Mailhog to catch all outgoing emails during development.
- **Performance**: Integrated Memcached support.
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
   docker-compose up -d
   ```
   On the first run, the `tlscerts` container will generate local certificates, and the `wordpress` container will download WordPress core and set up the database.

4. **Access the site**:
   Once the containers are running, navigate to the `LOCAL_DOMAIN` you configured (defaulting to the Nginx service aliases).

## Service Overview

- **Web (Nginx)**: Handles requests on ports `80` (HTTP) and `443` (HTTPS).
- **WordPress (PHP-FPM)**: The main PHP container (running as user 1000).
- **Database (MariaDB)**: Persistent database storage.
- **Adminer**: Database management UI available at `http://localhost:1080`.
- **Mailhog**: Email testing tool. UI available at `http://localhost:8025`.
- **Memcached**: Object caching service.

## Configuration

### Environment Variables
The environment is configured via `.env` (generated) and `env-values.toml`. Key variables include:

- `LOCAL_DOMAIN`: The local development URL (e.g., `project.test`).
- `WP_DB_NAME`, `WP_DB_USER`, `WP_DB_PASSWORD`: Database credentials (default: `wordpress`).
- `WP_IMAGE`: The Docker image for the WordPress service.
- `MULTISITE`: Set to `true` to enable WordPress Multisite (Subdomain install).
- `WP_DEBUG`: Boolean to toggle WordPress debug mode.

### PHP Configuration
Custom PHP settings can be adjusted in `php.ini`. The default configuration is optimized for development:
- `memory_limit`: 256M
- `upload_max_filesize`: 50M
- `post_max_size`: 75M
- **Xdebug**: Pre-configured to connect to `host.docker.internal` in `develop` mode.

### File Structure
- `wp-content/`: This directory is mapped to the container and contains your themes, plugins, and uploads.
- `.config/`: Contains Nginx configurations, entrypoint scripts, and helper scripts.
- `.jcore/`: Internal storage for WordPress core files, SSL certificates, and SQL dumps.

## Helper Scripts

The project includes several scripts located in `.config/scripts/` to streamline development workflows. These can be executed inside the `wordpress` container:

- `importdb`: Imports a database dump from `.jcore/sql`.
- `importmedia`: Syncs or imports media assets.
- `importplugins` / `importthemes`: Helpers for managing project dependencies.

Example usage:
```bash
docker-compose exec wordpress /project/.config/scripts/importdb
```

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
docker-compose exec wordpress wp plugin list
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
