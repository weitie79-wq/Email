package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr                            string
	DBDriver                        string
	DBDSN                           string
	DBPath                          string
	DBMaxOpenConns                  int
	DBMaxIdleConns                  int
	DBConnMaxLifetimeSeconds        int
	DBConnMaxIdleTimeSeconds        int
	DBConnectTimeoutSeconds         int
	DataDir                         string
	CookieName                      string
	SessionTTLHours                 int
	AdminEmail                      string
	AdminPassword                   string
	PublicHostname                  string
	PublicBaseURL                   string
	SMTPHost                        string
	SMTPPort                        string
	SMTPUsername                    string
	SMTPPassword                    string
	SMTPRequireTLS                  bool
	SubmissionAddr                  string
	SubmissionTLSAddr               string
	SubmissionMaxMessageMB          int
	TLSCertFile                     string
	TLSKeyFile                      string
	MaildirRoot                     string
	MaildirScanSeconds              int
	AllowInsecureHTTP               bool
	OpenRegistration                bool
	TwoFactorEnabled                bool
	TurnstileEnabled                bool
	TurnstileSiteKey                string
	TurnstileSecretKey              string
	CatchAllEnabled                 bool
	MailAutoRefresh                 bool
	MailRefreshSeconds              int
	UserMailboxApplyEnabled         bool
	UserMailboxDomainIDs            string
	ReservedMailboxPrefixes         string
	ExternalIMAPEnabled             bool
	ExternalIMAPSecretKey           string
	ExternalIMAPSyncSeconds         int
	ExternalIMAPAllowPrivateHosts   bool
	ExternalIMAPGmailClientID       string
	ExternalIMAPGmailClientSecret   string
	ExternalIMAPOutlookClientID     string
	ExternalIMAPOutlookClientSecret string
	MailTranslateEnabled            bool
	MailTranslateMaxChars           int
	DeliveryWebhookSecret           string
	StatusWebhookURL                string
	StatusWebhookSecret             string
	StatusWebhookAllowPrivateHosts  bool
	TelegramBotToken                string
	TelegramBotTimeoutSeconds       int
	TelegramAdminChatIDs            string
	TelegramNotifyEnabled           bool
	TelegramAlertQueueThreshold     int
}

func LoadConfig() Config {
	dataDir := getenv("EOOS_DATA_DIR", "./data")
	databaseDSN := getenv("EOOS_DATABASE_URL", getenv("EOOS_DB_DSN", ""))
	return Config{
		Addr:                            getenv("EOOS_ADDR", ":8080"),
		DBDriver:                        strings.ToLower(getenv("EOOS_DB_DRIVER", databaseDriverSQLite)),
		DBDSN:                           databaseDSN,
		DBPath:                          getenv("EOOS_DB_PATH", filepath.Join(dataDir, "eoos.db")),
		DBMaxOpenConns:                  getenvInt("EOOS_DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConns:                  getenvInt("EOOS_DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetimeSeconds:        getenvInt("EOOS_DB_CONN_MAX_LIFETIME_SECONDS", 1800),
		DBConnMaxIdleTimeSeconds:        getenvInt("EOOS_DB_CONN_MAX_IDLE_TIME_SECONDS", 300),
		DBConnectTimeoutSeconds:         getenvInt("EOOS_DB_CONNECT_TIMEOUT_SECONDS", 10),
		DataDir:                         dataDir,
		CookieName:                      getenv("EOOS_COOKIE_NAME", "eoos_session"),
		SessionTTLHours:                 getenvInt("EOOS_SESSION_TTL_HOURS", 24*7),
		AdminEmail:                      strings.ToLower(getenv("EOOS_ADMIN_EMAIL", "admin@eoos.local")),
		AdminPassword:                   getenv("EOOS_ADMIN_PASSWORD", ""),
		PublicHostname:                  getenv("EOOS_PUBLIC_HOSTNAME", "mail.eoos.local"),
		PublicBaseURL:                   getenv("EOOS_PUBLIC_BASE_URL", "http://localhost:5173"),
		SMTPHost:                        getenv("EOOS_SMTP_HOST", ""),
		SMTPPort:                        getenv("EOOS_SMTP_PORT", "25"),
		SMTPUsername:                    getenv("EOOS_SMTP_USERNAME", ""),
		SMTPPassword:                    getenv("EOOS_SMTP_PASSWORD", ""),
		SMTPRequireTLS:                  getenvBool("EOOS_SMTP_REQUIRE_TLS", false),
		SubmissionAddr:                  getenv("EOOS_SUBMISSION_ADDR", ""),
		SubmissionTLSAddr:               getenv("EOOS_SUBMISSION_TLS_ADDR", ""),
		SubmissionMaxMessageMB:          getenvInt("EOOS_SUBMISSION_MAX_MESSAGE_MB", 35),
		TLSCertFile:                     getenv("EOOS_TLS_CERT_FILE", ""),
		TLSKeyFile:                      getenv("EOOS_TLS_KEY_FILE", ""),
		MaildirRoot:                     getenv("EOOS_MAILDIR_ROOT", ""),
		MaildirScanSeconds:              getenvInt("EOOS_MAILDIR_SCAN_SECONDS", 30),
		AllowInsecureHTTP:               getenvBool("EOOS_ALLOW_INSECURE_HTTP", true),
		OpenRegistration:                getenvBool("EOOS_OPEN_REGISTRATION", false),
		TwoFactorEnabled:                getenvBool("EOOS_TWO_FACTOR_ENABLED", false),
		TurnstileEnabled:                getenvBool("EOOS_TURNSTILE_ENABLED", false),
		TurnstileSiteKey:                getenv("EOOS_TURNSTILE_SITE_KEY", ""),
		TurnstileSecretKey:              getenv("EOOS_TURNSTILE_SECRET_KEY", ""),
		CatchAllEnabled:                 getenvBool("EOOS_CATCH_ALL_ENABLED", false),
		MailAutoRefresh:                 getenvBool("EOOS_MAIL_AUTO_REFRESH", true),
		MailRefreshSeconds:              getenvInt("EOOS_MAIL_REFRESH_SECONDS", 30),
		UserMailboxApplyEnabled:         getenvBool("EOOS_USER_MAILBOX_APPLY_ENABLED", false),
		UserMailboxDomainIDs:            getenv("EOOS_USER_MAILBOX_DOMAIN_IDS", ""),
		ReservedMailboxPrefixes:         getenv("EOOS_RESERVED_MAILBOX_PREFIXES", "admin,postmaster,abuse,hostmaster,webmaster,root,security,noreply,no-reply,mailer-daemon"),
		ExternalIMAPEnabled:             getenvBool("EOOS_EXTERNAL_IMAP_ENABLED", false),
		ExternalIMAPSecretKey:           getenv("EOOS_EXTERNAL_IMAP_SECRET_KEY", ""),
		ExternalIMAPSyncSeconds:         getenvInt("EOOS_EXTERNAL_IMAP_SYNC_SECONDS", 300),
		ExternalIMAPAllowPrivateHosts:   getenvBool("EOOS_EXTERNAL_IMAP_ALLOW_PRIVATE_HOSTS", false),
		ExternalIMAPGmailClientID:       getenv("EOOS_EXTERNAL_IMAP_GMAIL_CLIENT_ID", ""),
		ExternalIMAPGmailClientSecret:   getenv("EOOS_EXTERNAL_IMAP_GMAIL_CLIENT_SECRET", ""),
		ExternalIMAPOutlookClientID:     getenv("EOOS_EXTERNAL_IMAP_OUTLOOK_CLIENT_ID", ""),
		ExternalIMAPOutlookClientSecret: getenv("EOOS_EXTERNAL_IMAP_OUTLOOK_CLIENT_SECRET", ""),
		MailTranslateEnabled:            getenvBool("EOOS_MAIL_TRANSLATE_ENABLED", true),
		MailTranslateMaxChars:           getenvInt("EOOS_MAIL_TRANSLATE_MAX_CHARS", 8000),
		DeliveryWebhookSecret:           getenv("EOOS_DELIVERY_WEBHOOK_SECRET", ""),
		StatusWebhookURL:                getenv("EOOS_STATUS_WEBHOOK_URL", ""),
		StatusWebhookSecret:             getenv("EOOS_STATUS_WEBHOOK_SECRET", ""),
		StatusWebhookAllowPrivateHosts:  getenvBool("EOOS_STATUS_WEBHOOK_ALLOW_PRIVATE_HOSTS", false),
		TelegramBotToken:                getenv("EOOS_TELEGRAM_BOT_TOKEN", ""),
		TelegramBotTimeoutSeconds:       getenvInt("EOOS_TELEGRAM_BOT_TIMEOUT_SECONDS", 60),
		TelegramAdminChatIDs:            getenv("EOOS_TELEGRAM_ADMIN_CHAT_IDS", ""),
		TelegramNotifyEnabled:           getenvBool("EOOS_TELEGRAM_NOTIFY_ENABLED", true),
		TelegramAlertQueueThreshold:     getenvInt("EOOS_TELEGRAM_ALERT_QUEUE_THRESHOLD", 200),
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	var n int
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
