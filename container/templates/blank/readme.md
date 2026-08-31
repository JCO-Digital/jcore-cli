# WordPress Blank Project

This is a clean WordPress project scaffolded using the `blank` template from the JCORE WordPress Container.

## Getting Started

1.  **Environment Setup**: Ensure your `.env` file is configured correctly in the project root.
2.  **Start Containers**: Run `docker-compose up -d`.
3.  **Development**: Place your custom themes and plugins in the `wp-content/` directory.

## Features

- Minimal configuration.
- Standard WordPress structure.
- Compatible with JCORE container helper scripts.

## Scripts

You can use the helper scripts provided by the container for common tasks:
- `docker-compose exec wordpress /project/.config/scripts/importdb` - Import database.
- `docker-compose exec wordpress /project/.config/scripts/importmedia` - Import media files.
