# Session Store E2E Tests

This directory contains end-to-end tests for database integrations with the session store.

## Running with Docker Compose

A Docker Compose configuration is provided to easily run the required database services for testing:

1. Start the database services:
   ```
   docker-compose up -d
   ```

2. Run the tests with the appropriate environment variables:
   ```
   WEB_TEST_POSTGRESQL_URL="postgres://test_user:test_password@localhost:5438/test_db" \
   WEB_TEST_MYSQL_URL="test_user:test_password@tcp(localhost:3308)/test_db?tls=skip-verify" \
   go test -v
   ```

3. Stop the services when done:
   ```
   docker-compose down
   ```

To reset all data, you can use:
```
docker-compose down -v
```
