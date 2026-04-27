-- +goose Up
-- Initialize logical schemas so later migrations can target them independently.
CREATE SCHEMA IF NOT EXISTS iam;
CREATE SCHEMA IF NOT EXISTS wallet;
CREATE SCHEMA IF NOT EXISTS catalog;
CREATE SCHEMA IF NOT EXISTS pool;
CREATE SCHEMA IF NOT EXISTS metering;
CREATE SCHEMA IF NOT EXISTS audit;

-- Enable extensions we rely on. TimescaleDB is required for the metering schema.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "timescaledb";

-- +goose Down
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS metering CASCADE;
DROP SCHEMA IF EXISTS pool CASCADE;
DROP SCHEMA IF EXISTS catalog CASCADE;
DROP SCHEMA IF EXISTS wallet CASCADE;
DROP SCHEMA IF EXISTS iam CASCADE;
