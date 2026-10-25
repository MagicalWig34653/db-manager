-- DB Manager SQL export 1; same-dialect tables only
-- Snapshot: per database; no server-wide snapshot. See EXPORT-FORMAT.
-- Catalog reconstruction: omits sequences, triggers, rules, partitions, ownership and storage.
SET standard_conforming_strings=on;
SET TIME ZONE 'UTC';
CREATE TABLE "public"."ex""` port';--" (
  "id" bigint,
  "text_value" text,
  "empty_value" text,
  "nil_value" text,
  "bytes_value" bytea,
  "exact_value" numeric(40,20),
  "utc_value" timestamp with time zone
);
CREATE INDEX "export index" ON public."ex""` port';--" USING btree (id);
INSERT INTO "public"."ex""` port';--" ("id","text_value","empty_value","nil_value","bytes_value","exact_value","utc_value") OVERRIDING SYSTEM VALUE VALUES (9007199254740993,E'Grüße ''雪, line
=1+1',E'',NULL,decode('00ff27225c','hex'),12345678901234567890.12345678901234567890,E'2026-01-02T03:04:05.123456Z');
-- DBMANAGER EXPORT COMPLETE
