# JCORE2 Project

This project is based on the JCORE2 WordPress template. It uses a child theme approach with the `jcore2` base theme included as a git submodule.

## Features

- **JCORE2 Child Theme**: Custom styling and functionality built on top of the robust JCORE2 framework.
- **Git Submodules**: The base theme is managed as a submodule for easy updates.
- **Build System**: Includes a `Makefile` and `build.mjs` for streamlining development tasks.
- **Pre-configured Tooling**: Comes with Prettier and Composer configurations out of the box.

## Development

### Prerequisites

Ensure the parent `wordpress-container` is set up and running.

### Installation

1. Initialize and update submodules:
   ```bash
   git submodule update --init --recursive
   ```

2. Install dependencies:
   ```bash
   pnpm install
   composer install
   ```

### Building

Use the provided Makefile for common tasks:
```bash
make build
```

## Structure

- `wp-content/themes/jcore2`: The base theme (submodule).
- `wp-content/themes/<child-theme>`: Your custom child theme.
- `defaults.toml`: Template-specific configuration values.

## License

This project is licensed under the GPL.
