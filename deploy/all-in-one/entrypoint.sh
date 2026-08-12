#!/bin/sh
set -eu

: "${EOOS_PUBLIC_HOSTNAME:=mail.example.com}"
: "${EOOS_DATA_DIR:=/data}"
: "${EOOS_DB_PATH:=/data/eoos.db}"
: "${EOOS_DB_DRIVER:=sqlite}"
: "${EOOS_ADDR:=127.0.0.1:8080}"
: "${EOOS_SMTP_HOST:=127.0.0.1}"
: "${EOOS_SMTP_PORT:=25}"
: "${EOOS_SUBMISSION_ADDR:=}"
: "${EOOS_SUBMISSION_TLS_ADDR:=}"
: "${EOOS_SUBMISSION_MAX_MESSAGE_MB:=35}"
: "${EOOS_MAILDIR_ROOT:=/var/mail/vhosts}"
: "${EOOS_TLS_CERT_FILE:=}"
: "${EOOS_TLS_KEY_FILE:=}"

export EOOS_DATA_DIR EOOS_DB_PATH EOOS_DB_DRIVER EOOS_ADDR EOOS_SMTP_HOST EOOS_SMTP_PORT EOOS_SUBMISSION_ADDR EOOS_SUBMISSION_TLS_ADDR EOOS_SUBMISSION_MAX_MESSAGE_MB EOOS_MAILDIR_ROOT EOOS_TLS_CERT_FILE EOOS_TLS_KEY_FILE

addgroup --system --gid 5000 vmail 2>/dev/null || true
adduser --system --uid 5000 --gid 5000 --home /var/mail/vhosts --no-create-home vmail 2>/dev/null || true
mkdir -p /data /var/mail/vhosts /var/lib/rspamd/dkim /run/rspamd /var/spool/postfix /var/run/dovecot
chown -R 5000:5000 /var/mail/vhosts
if id _rspamd >/dev/null 2>&1; then
	chown -R _rspamd:_rspamd /run/rspamd /var/lib/rspamd 2>/dev/null || true
elif id rspamd >/dev/null 2>&1; then
	chown -R rspamd:rspamd /run/rspamd /var/lib/rspamd 2>/dev/null || true
fi

AUTH_POLICY_NONCE_FILE="${EOOS_AUTH_POLICY_NONCE_FILE:-/data/dovecot-auth-policy-nonce}"
mkdir -p "$(dirname "$AUTH_POLICY_NONCE_FILE")"
if [ ! -s "$AUTH_POLICY_NONCE_FILE" ]; then
	od -An -tx1 -N32 /dev/urandom | tr -d ' \n' >"$AUTH_POLICY_NONCE_FILE"
fi
chmod 600 "$AUTH_POLICY_NONCE_FILE" 2>/dev/null || true
AUTH_POLICY_HASH_NONCE="$(cat "$AUTH_POLICY_NONCE_FILE")"

/usr/local/bin/eoos-postfix-configure-db
/usr/local/bin/eoos-dovecot-configure-db

TLS_CERT=/etc/ssl/certs/ssl-cert-snakeoil.pem
TLS_KEY=/etc/ssl/private/ssl-cert-snakeoil.key
if [ -n "$EOOS_TLS_CERT_FILE" ] || [ -n "$EOOS_TLS_KEY_FILE" ]; then
	if [ -f "$EOOS_TLS_CERT_FILE" ] && [ -f "$EOOS_TLS_KEY_FILE" ]; then
		TLS_CERT="$EOOS_TLS_CERT_FILE"
		TLS_KEY="$EOOS_TLS_KEY_FILE"
		: "${EOOS_SUBMISSION_ADDR:=:587}"
		: "${EOOS_SUBMISSION_TLS_ADDR:=:465}"
	else
		echo "warning: EOOS_TLS_CERT_FILE/EOOS_TLS_KEY_FILE not readable; using snakeoil localhost certificate" >&2
	fi
fi
if [ -n "$EOOS_SUBMISSION_ADDR$EOOS_SUBMISSION_TLS_ADDR" ] && { [ "$TLS_CERT" = "/etc/ssl/certs/ssl-cert-snakeoil.pem" ] || [ "$TLS_KEY" = "/etc/ssl/private/ssl-cert-snakeoil.key" ]; }; then
	echo "warning: SMTP submission disabled because EOOS_TLS_CERT_FILE/EOOS_TLS_KEY_FILE are not configured with readable certificate files" >&2
	EOOS_SUBMISSION_ADDR=""
	EOOS_SUBMISSION_TLS_ADDR=""
fi
export EOOS_SUBMISSION_ADDR EOOS_SUBMISSION_TLS_ADDR

postconf -e "myhostname = ${EOOS_PUBLIC_HOSTNAME}"
postconf -e "myorigin = ${EOOS_PUBLIC_HOSTNAME}"
postconf -e "smtpd_tls_cert_file = ${TLS_CERT}"
postconf -e "smtpd_tls_key_file = ${TLS_KEY}"
postconf -e "virtual_transport = lmtp:inet:127.0.0.1:24"
postconf -e "milter_mail_macros = i {mail_addr} {client_addr} {client_name} {auth_authen}"
postconf -e "smtpd_milters = inet:127.0.0.1:11332"
postconf -e "non_smtpd_milters = inet:127.0.0.1:11332"
sed -i "s#^ssl_cert = <.*#ssl_cert = <${TLS_CERT}#" /etc/dovecot/dovecot.conf
sed -i "s#^ssl_key = <.*#ssl_key = <${TLS_KEY}#" /etc/dovecot/dovecot.conf
sed -i "s#^auth_policy_hash_nonce = .*#auth_policy_hash_nonce = ${AUTH_POLICY_HASH_NONCE}#" /etc/dovecot/dovecot.conf

# Rspamd DKIM keys are exported after API seed/migrations are complete.
/usr/local/bin/eoos-api >/tmp/eoos-api-bootstrap.log 2>&1 &
bootstrap_pid=$!
bootstrap_ready=0
for i in $(seq 1 60); do
	if curl --fail --silent --show-error http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
		bootstrap_ready=1
		break
	fi
	sleep 1
done
if [ "$bootstrap_ready" -ne 1 ]; then
	echo "error: EOOS API database bootstrap did not become ready" >&2
	cat /tmp/eoos-api-bootstrap.log >&2 || true
	kill "$bootstrap_pid" 2>/dev/null || true
	wait "$bootstrap_pid" 2>/dev/null || true
	exit 1
fi
kill "$bootstrap_pid" 2>/dev/null || true
wait "$bootstrap_pid" 2>/dev/null || true

/usr/local/bin/eoos-rspamd-sync-dkim --once || true

postfix check
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/eoos.conf
