-- Runs once on a fresh Postgres volume (docker-entrypoint-initdb.d). Creates the
-- dedicated test database so `go test` never truncates dev data in `kova`.
CREATE DATABASE kova_test OWNER kova;
