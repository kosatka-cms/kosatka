#!/usr/bin/env bash
# Compares a site's MySQL schema with the one the ent schemas create
# (columns, types, nullability, defaults, indexes by columns, foreign keys
# and their ON DELETE). Read-only. Prints the differences; none (apart
# from MySQL-only ON UPDATE CURRENT_TIMESTAMP) means ent describes the
# site's database exactly. The database is the one the site's settings
# name (DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME; an SSH tunnel for
# a remote one); the ent schema comes from scripts/test_db.sh (Docker).
#   DB_HOST=127.0.0.1 DB_USER=… DB_PASSWORD=… DB_NAME=… ./scripts/schema_drift.sh
set -euo pipefail
cd "$(dirname "$0")/.."
for v in DB_HOST DB_USER DB_NAME; do
  if [[ -z "${!v:-}" ]]; then
    echo "✗ ${v} is not set" >&2
    exit 1
  fi
done
out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT

SCHEMA_DUMP="$out/ent_all.tsv" ./scripts/test_db.sh -run TestDumpSchema ./internal/infrastructure/ent/schematest/ >/dev/null

q="SELECT 'C', table_name, column_name, column_type, is_nullable, COALESCE(column_default,'<null>'), extra FROM information_schema.columns WHERE table_schema = DATABASE() ORDER BY table_name, ordinal_position;
SELECT 'I', table_name, GROUP_CONCAT(column_name ORDER BY seq_in_index), MAX(non_unique), index_name, MAX(index_type) FROM information_schema.statistics WHERE table_schema = DATABASE() GROUP BY table_name, index_name ORDER BY 1,2;
SELECT 'F', k.table_name, k.column_name, k.referenced_table_name, k.referenced_column_name, r.delete_rule, k.constraint_name FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.constraint_name=k.constraint_name WHERE k.table_schema = DATABASE() AND k.referenced_table_name IS NOT NULL ORDER BY 2,3"
MYSQL_PWD="${DB_PASSWORD:-}" mysql -h "$DB_HOST" -P "${DB_PORT:-3306}" -u "$DB_USER" "$DB_NAME" -N -e "$q" > "$out/db_all.tsv"

python3 scripts/schema_drift/diff.py "$out"
