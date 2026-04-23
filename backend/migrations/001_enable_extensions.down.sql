-- backend/migrations/001_enable_extensions.down.sql
DROP EXTENSION IF EXISTS "postgis";
DROP EXTENSION IF EXISTS "pgcrypto";
DROP EXTENSION IF EXISTS "uuid-ossp";
