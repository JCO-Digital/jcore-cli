# JCORE1 Project

This project is based on the JCORE1 template, which uses a child theme structure with the original JCORE theme as a git submodule.

## Development Environment

This project is designed to run within the [WordPress Container](https://github.com/JCO-Digital/wordpress-container) environment.

### Theme Structure

- **Base Theme**: JCORE (located in `wp-content/themes/jcore` as a submodule).
- **Child Theme**: Custom styles and functionality are implemented in the child theme directory within `wp-content/themes/`.

## Getting Started

1. Ensure the parent [wordpress-container](..) is set up.
2. The `jcore` submodule should be automatically initialized. If not, run:
   ```bash
   git submodule update --init --recursive
   ```
3. Use `npm` or `pnpm` within the theme directory for asset compilation if applicable.

## Tools and Commands

Refer to the main project documentation for database imports and general container management.
