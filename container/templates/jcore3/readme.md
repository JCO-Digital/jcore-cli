# JCORE3 WordPress Project

This project is based on the **JCORE3** template, a modern WordPress development workflow featuring the JCORE Hurricane theme and Lohko blocks.

## Features

- **Hurricane Theme**: Uses the JCORE "Hurricane" branch for a robust, performant base.
- **Lohko Blocks**: Integrated support for Lohko block-based editing.
- **Modern Tooling**: Pre-configured with Composer for PHP dependencies and pnpm for asset management.
- **Linting & Formatting**: Standardized WordPress coding styles via Prettier and PHPCS.

## Development

### Setup

1.  Project settings are configured in the root `jcore.toml`.
2.  Install dependencies:
    ```bash
    composer install
    pnpm install
    ```

### Building Assets

Use the included `Makefile` or npm scripts:
```bash
# Build production assets
pnpm run build

# Start development watcher
pnpm run dev
```

## Structure

- `wp-content/themes/jcore-ilme`: The main theme directory (cloned from the Hurricane branch).
- `composer.json`: PHP dependency management.
- `package.json`: Node.js dependency management.

## Deployment

Refer to the JCO Digital deployment guidelines for pushing to staging or production environments.
