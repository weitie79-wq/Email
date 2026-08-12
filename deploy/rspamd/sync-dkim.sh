#!/bin/sh
set -eu

: "${EOOS_DB_PATH:=/data/eoos.db}"
: "${EOOS_DB_DRIVER:=sqlite}"
: "${EOOS_DB_HOST:=}"
: "${EOOS_DB_PORT:=}"
: "${EOOS_DB_NAME:=}"
: "${EOOS_DB_USER:=}"
: "${EOOS_DB_PASSWORD:=}"
: "${EOOS_RSPAMD_DKIM_DIR:=/var/lib/rspamd/dkim}"
: "${EOOS_RSPAMD_DKIM_SYNC_SECONDS:=60}"

chown_dkim_dir() {
	if id _rspamd >/dev/null 2>&1; then
		chown -R _rspamd:_rspamd "$EOOS_RSPAMD_DKIM_DIR" 2>/dev/null || true
	elif id rspamd >/dev/null 2>&1; then
		chown -R rspamd:rspamd "$EOOS_RSPAMD_DKIM_DIR" 2>/dev/null || true
	fi
}

sync_keys() {
	mkdir -p "$EOOS_RSPAMD_DKIM_DIR"
	rows_file="$(mktemp)"
	chmod 0600 "$rows_file"
	trap 'rm -f "$rows_file"' EXIT HUP INT TERM
	case "$(printf '%s' "$EOOS_DB_DRIVER" | tr '[:upper:]' '[:lower:]')" in
	'' | sqlite | sqlite3)
		if [ ! -f "$EOOS_DB_PATH" ]; then
			rm -f "$rows_file"
			trap - EXIT HUP INT TERM
			chown_dkim_dir
			return 0
		fi
		sqlite3 -separator '|' "$EOOS_DB_PATH" "SELECT name,dkim_selector,dkim_private_key FROM domains WHERE status='active';" >"$rows_file"
		;;
	mysql)
		MYSQL_PWD="$EOOS_DB_PASSWORD" mysql --protocol=TCP --host="$EOOS_DB_HOST" --port="$EOOS_DB_PORT" --user="$EOOS_DB_USER" --database="$EOOS_DB_NAME" --batch --raw --skip-column-names --execute="SELECT CONCAT_WS('|',name,dkim_selector,dkim_private_key) FROM domains WHERE status='active'" >"$rows_file"
		;;
	pg | pgsql | postgres | postgresql)
		PGPASSWORD="$EOOS_DB_PASSWORD" psql --host="$EOOS_DB_HOST" --port="$EOOS_DB_PORT" --username="$EOOS_DB_USER" --dbname="$EOOS_DB_NAME" --no-align --tuples-only --field-separator='|' --command="SELECT name,dkim_selector,dkim_private_key FROM domains WHERE status='active'" >"$rows_file"
		;;
	*)
		echo "error: unsupported EOOS_DB_DRIVER=$EOOS_DB_DRIVER" >&2
		return 1
		;;
	esac

	while IFS='|' read -r domain selector private_key; do
		[ -n "$domain" ] || continue
		[ -n "$selector" ] || selector="eoos"
		keyfile="$EOOS_RSPAMD_DKIM_DIR/${domain}.${selector}.key"
		tmpfile="${keyfile}.tmp"
		printf '%s' "$private_key" | base64 -d >"$tmpfile"
		chmod 0640 "$tmpfile"
		mv "$tmpfile" "$keyfile"
	done <"$rows_file"

	rm -f "$rows_file"
	trap - EXIT HUP INT TERM

	chown_dkim_dir
}

if [ "${1:-}" = "--once" ]; then
	sync_keys
	exit 0
fi

while true; do
	sync_keys || true
	sleep "$EOOS_RSPAMD_DKIM_SYNC_SECONDS"
done
