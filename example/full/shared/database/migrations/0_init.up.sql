CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE "user" (
  id_user uuid default gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  PRIMARY KEY(id_user)
);