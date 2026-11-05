package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// The environments a site runs in (env / APP_ENV): LoadPath accepts these
// two only.
const (
	// EnvProd - a live site, the default: templates cached, the modules'
	// files the binary's, a session key required, TLS to a remote
	// database.
	EnvProd = "prod"
	// EnvLocal - development: a debug log, templates read again on every
	// request, the modules' files from their source directories first,
	// no TLS default for a remote database.
	EnvLocal = "local"
)

// Config - the site's configuration: the YAML file Load reads, with the
// environment variables (the env tags) over its values. Env - the
// environment, EnvProd (the default) or EnvLocal; Port - the port the
// site's HTTP server listens on.
type Config struct {
	Env       string          `yaml:"env" env:"APP_ENV" env-default:"prod"`
	Port      int             `yaml:"port" env:"APP_PORT"`
	DB        DBConfig        `yaml:"db"`
	Session   SessionConfig   `yaml:"session"`
	Timeouts  TimeoutConfig   `yaml:"timeouts"`
	Email     EmailConfig     `yaml:"email"`
	Recaptcha RecaptchaConfig `yaml:"recaptcha"`
	OAuth     OAuthConfig     `yaml:"oauth"`
	// SecretsKey - the key the secrets kept in the database are sealed
	// with (the outside services' keys saved in the admin, the users'
	// two-factor secrets; internal/lib/secretbox): a long random string. "new,old"
	// while replacing it - the old one still opens what it sealed. Empty -
	// they are kept unsealed.
	SecretsKey string `yaml:"secrets_key" env:"SECRETS_KEY"`
	// PublicBaseURL - the site's canonical origin (e.g. https://www.example.com);
	// when set, it overrides the one saved in Site information (service.SiteURL),
	// so a copy of the production database keeps this site's URL. Empty - the
	// admin's, else derived from the request.
	PublicBaseURL string `yaml:"public_base_url" env:"PUBLIC_BASE_URL"`
	// PageCacheMaxAge - seconds a shared cache (a CDN in front of the site)
	// may keep a guest's page (middleware.PageCacheMiddleware: Cache-Control
	// "public, s-maxage=…" on guests' HTML, "private, no-store" on the rest).
	// 0 - off.
	PageCacheMaxAge int `yaml:"page_cache_max_age" env:"PAGE_CACHE_MAX_AGE"`
	// TrustedProxies - the proxies in front of the site whose
	// X-Forwarded-For / X-Real-IP give the visitor's address (addresses or
	// networks: 10.0.0.5, 172.16.0.0/12; httputil.ClientIP). Empty -
	// loopback only, a web server on the same host.
	TrustedProxies []string `yaml:"trusted_proxies" env:"TRUSTED_PROXIES" env-separator:","`
	// Theme - the site theme: a directory under themes/ (see
	// paths.SetTheme). Empty = engine templates only.
	Theme string `yaml:"theme" env:"THEME"`
	// ThemesCatalogURL - the themes' catalog (an index.json built by app
	// theme index from a list of the themes' repositories): the Appearance
	// page's catalog and its updates. The Kosatka catalog by default;
	// "off" (or THEMES_CATALOG_URL set empty) - no catalog.
	ThemesCatalogURL string `yaml:"themes_catalog_url" env:"THEMES_CATALOG_URL" env-default:"https://kosatka-cms.github.io/themes/index.json"`
	// TranslationsCatalogURL - the translations catalog (an index.json of
	// the interface's languages to download, their files next to it): the
	// Interface translations page's Languages tab, app install -language.
	// The Kosatka catalog by default; "off" (or TRANSLATIONS_CATALOG_URL
	// set empty) - no catalog.
	TranslationsCatalogURL string `yaml:"translations_catalog_url" env:"TRANSLATIONS_CATALOG_URL" env-default:"https://kosatka-cms.github.io/translations/index.json"`
	// TemplatesDebug - HTML comments in the pages naming the template
	// files that rendered them and the suggestions tried (like Twig debug,
	// service.DefaultTemplateService.SetDebug); for a theme's author, not
	// a live site.
	TemplatesDebug bool `yaml:"templates_debug" env:"TEMPLATES_DEBUG"`
	// Files - where public files (uploads) live and the URL path they are
	// served at (see paths.SetPublicFiles).
	Files FilesConfig `yaml:"files"`
	// EngineDir - where the engine's own files are when several sites share
	// one engine: <dir>/web/templates and <dir>/web/static; the site runs in
	// its own directory, with its config, themes and files (see
	// paths.SetEngineDir). Empty - the working directory's web/.
	EngineDir string `yaml:"engine_dir" env:"ENGINE_DIR"`
	// Content - the content's lifecycle.
	Content ContentConfig `yaml:"content"`
}

// ContentConfig - the content's lifecycle.
type ContentConfig struct {
	// TrashDays - how many days deleted content stays in the trash before
	// it is deleted for good (service.NodeDeleteService.TrashJob); 0 -
	// until the trash is emptied by hand.
	TrashDays int `yaml:"trash_days" env:"CONTENT_TRASH_DAYS" env-default:"30"`
}

// FilesConfig - the public files directory and its URL path. Empty - the
// defaults, ./web/uploads at /uploads/. Content stores the files' URLs:
// change the URL path only on a new site.
type FilesConfig struct {
	PublicDir string `yaml:"public_dir" env:"FILES_PUBLIC_DIR"`
	PublicURL string `yaml:"public_url" env:"FILES_PUBLIC_URL"`
	// PrivateDir - files never served as they are, only through the
	// application with a permission check (e.g. files sent with forms).
	// Empty - ./private next to the application.
	PrivateDir string `yaml:"private_dir" env:"FILES_PRIVATE_DIR"`
	// Storage - where the public files are kept: "local" (the default,
	// PublicDir), "s3" (an S3-compatible bucket, S3) or "azure" (an Azure
	// Blob Storage container, Azure).
	Storage string      `yaml:"storage" env:"FILES_STORAGE"`
	S3      S3Config    `yaml:"s3"`
	Azure   AzureConfig `yaml:"azure"`
	// PrivateStorage - where the private files are kept: "local" (the
	// default, PrivateDir), "s3" (PrivateS3's bucket, reached with S3's
	// endpoint and keys) or "azure" (PrivateAzure's container of Azure's
	// account).
	PrivateStorage string             `yaml:"private_storage" env:"FILES_PRIVATE_STORAGE"`
	PrivateS3      PrivateS3Config    `yaml:"private_s3"`
	PrivateAzure   PrivateAzureConfig `yaml:"private_azure"`
}

// AzureConfig - the container of files.storage "azure". The account's
// key comes only from the environment, never the config file.
type AzureConfig struct {
	Account   string `yaml:"account" env:"FILES_AZURE_ACCOUNT"`
	Container string `yaml:"container" env:"FILES_AZURE_CONTAINER"`
	// Prefix - the container's folder for this site's files ("" - its
	// root).
	Prefix string `yaml:"prefix" env:"FILES_AZURE_PREFIX"`
	// Endpoint - the account's blob address ("" -
	// https://<account>.blob.core.windows.net).
	Endpoint string `yaml:"endpoint" env:"FILES_AZURE_ENDPOINT"`
	// PublicURL - where the container (or a CDN in front of it) serves the
	// files itself: the site redirects its files' addresses there. Empty -
	// the site serves them from the container.
	PublicURL string `yaml:"public_url" env:"FILES_AZURE_PUBLIC_URL"`
	Key       string `yaml:"-" env:"FILES_AZURE_KEY"`
}

// PrivateAzureConfig - the container of files.private_storage "azure", in
// files.azure's account: one the public files' address never reaches.
type PrivateAzureConfig struct {
	Container string `yaml:"container" env:"FILES_PRIVATE_AZURE_CONTAINER"`
	Prefix    string `yaml:"prefix" env:"FILES_PRIVATE_AZURE_PREFIX"`
}

// PrivateS3Config - the bucket of files.private_storage "s3": one the
// public files' address never reaches (another bucket, or a prefix
// outside theirs).
type PrivateS3Config struct {
	Bucket string `yaml:"bucket" env:"FILES_PRIVATE_S3_BUCKET"`
	Prefix string `yaml:"prefix" env:"FILES_PRIVATE_S3_PREFIX"`
}

// S3Config - the bucket of files.storage "s3". The keys come only from
// the environment, never the config file.
type S3Config struct {
	// Endpoint - the storage's address, e.g. https://storage.yandexcloud.net.
	Endpoint string `yaml:"endpoint" env:"FILES_S3_ENDPOINT"`
	Region   string `yaml:"region" env:"FILES_S3_REGION"`
	Bucket   string `yaml:"bucket" env:"FILES_S3_BUCKET"`
	// Prefix - the bucket's folder for this site's files ("" - its root).
	Prefix string `yaml:"prefix" env:"FILES_S3_PREFIX"`
	// PublicURL - where the bucket (or a CDN in front of it) serves the
	// files itself: the site redirects its files' addresses there. Empty -
	// the site serves them from the bucket.
	PublicURL string `yaml:"public_url" env:"FILES_S3_PUBLIC_URL"`
	// PathStyle - <endpoint>/<bucket> addresses (MinIO, some providers).
	PathStyle bool   `yaml:"path_style" env:"FILES_S3_PATH_STYLE"`
	AccessKey string `yaml:"-" env:"FILES_S3_ACCESS_KEY"`
	SecretKey string `yaml:"-" env:"FILES_S3_SECRET_KEY"`
}

// RecaptchaConfig enables Google reCAPTCHA v2 on auth forms when both keys are non-empty.
type RecaptchaConfig struct {
	SiteKey   string `yaml:"site_key" env:"RECAPTCHA_SITE_KEY"`
	SecretKey string `yaml:"secret_key" env:"RECAPTCHA_SECRET_KEY"`
}

// OAuthConfig holds OAuth 2.0 client credentials per provider.
type OAuthConfig struct {
	Google GoogleOAuthConfig `yaml:"google"`
	VK     VKOAuthConfig     `yaml:"vk"`
	Yandex YandexOAuthConfig `yaml:"yandex"`
}

// GoogleOAuthConfig - the Google OAuth client's keys (oauth.google), used
// while the admin's social sign-in settings save none for Google.
type GoogleOAuthConfig struct {
	ClientID     string `yaml:"client_id" env:"GOOGLE_OAUTH_CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" env:"GOOGLE_OAUTH_CLIENT_SECRET"`
}

// VKOAuthConfig - the VK OAuth application's keys (oauth.vk), used while
// the admin's social sign-in settings save none for VK.
type VKOAuthConfig struct {
	ClientID     string `yaml:"client_id" env:"VK_OAUTH_CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" env:"VK_OAUTH_CLIENT_SECRET"`
}

// YandexOAuthConfig - the Yandex OAuth application's keys (oauth.yandex),
// used while the admin's social sign-in settings save none for Yandex.
type YandexOAuthConfig struct {
	ClientID     string `yaml:"client_id" env:"YANDEX_OAUTH_CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" env:"YANDEX_OAUTH_CLIENT_SECRET"`
}

// OAuthProviderConfig is a normalized view of provider credentials.
type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
}

// Enabled reports whether both keys are set: the provider can be used.
func (c OAuthProviderConfig) Enabled() bool {
	return strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
}

// Config - the keys as an OAuthProviderConfig.
func (c GoogleOAuthConfig) Config() OAuthProviderConfig {
	return OAuthProviderConfig(c)
}

// Config - the keys as an OAuthProviderConfig.
func (c VKOAuthConfig) Config() OAuthProviderConfig {
	return OAuthProviderConfig(c)
}

// Config - the keys as an OAuthProviderConfig.
func (c YandexOAuthConfig) Config() OAuthProviderConfig {
	return OAuthProviderConfig(c)
}

// EmailConfig - the outgoing mail's transport when the admin's mail
// settings name none: an SMTP server (host, port, account) or a Resend
// API key, and the sender's address.
type EmailConfig struct {
	SMTPHost     string `yaml:"smtp_host" env:"SMTP_HOST"`
	SMTPPort     int    `yaml:"smtp_port" env:"SMTP_PORT" env-default:"587"`
	SMTPUser     string `yaml:"smtp_user" env:"SMTP_USER"`
	SMTPPassword string `yaml:"smtp_password" env:"SMTP_PASSWORD"`
	FromAddress  string `yaml:"from_address" env:"EMAIL_FROM_ADDRESS"`
	// ResendAPIKey enables HTTPS API sending (https://resend.com). If set with from_address, used instead of SMTP.
	ResendAPIKey string `yaml:"resend_api_key" env:"RESEND_API_KEY"`
}

// DBConfig - the site database's connection: the server (Driver, Host,
// Port - the driver's usual one when 0), the database (Scheme), the
// account, the pool's sizes and connection lifetimes, TLS and MySQL's
// session mode.
type DBConfig struct {
	// Driver - the database server: DriverMySQL (default) or DriverPostgres.
	Driver string `yaml:"driver" env:"DB_DRIVER" env-default:"mysql"`
	Host   string `yaml:"host" env:"DB_HOST"`
	// Scheme is the database name (yaml key "scheme" for backward compatibility).
	Scheme       string `yaml:"scheme" env:"DB_NAME"`
	Username     string `yaml:"username" env:"DB_USER"`
	Password     string `yaml:"password" env:"DB_PASSWORD"`
	Port         int    `yaml:"port" env:"DB_PORT"`
	MaxOpenConns int    `yaml:"max_open_conns" env:"DB_MAX_OPEN_CONNS" env-default:"25"`
	MaxIdleConns int    `yaml:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" env-default:"5"`
	// ConnMaxLifetime / ConnMaxIdleTime - minutes before a pooled connection
	// is recycled (before the server drops it) or closed when idle.
	ConnMaxLifetime int `yaml:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" env-default:"5"`
	ConnMaxIdleTime int `yaml:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME" env-default:"5"`
	// TLS - MySQL: go-sql-driver/mysql's tls=... (e.g. true, skip-verify,
	// preferred), needed when the server uses require_secure_transport=ON
	// (many managed MySQL hosts). PostgreSQL: the sslmode (disable,
	// require, verify-full, …).
	TLS string `yaml:"tls" env:"DB_TLS"`
	// SQLMode - MySQL only: session sql_mode set on every connection (the server's
	// global mode is left untouched). Strict by default, so over-long strings
	// or missing NOT NULL values fail instead of being silently coerced.
	// SQLModeServerDefault ("server") keeps the server's global mode.
	SQLMode string `yaml:"sql_mode" env:"DB_SQL_MODE" env-default:"STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"`
}

// SQLModeServerDefault - DBConfig.SQLMode value meaning "don't set a
// session sql_mode".
const SQLModeServerDefault = "server"

// The database servers DBConfig.Driver names.
const (
	DriverMySQL    = "mysql"
	DriverPostgres = "postgres"
)

// IsPostgres - the site database is PostgreSQL.
func (c DBConfig) IsPostgres() bool {
	return strings.EqualFold(strings.TrimSpace(c.Driver), DriverPostgres)
}

// Dialect - the database's SQL dialect name: DriverMySQL or DriverPostgres
// (the names ent's dialect and sqldialect use).
func (c DBConfig) Dialect() string {
	if c.IsPostgres() {
		return DriverPostgres
	}
	return DriverMySQL
}

// SQLDriver - the database/sql driver name DSN is for.
func (c DBConfig) SQLDriver() string {
	if c.IsPostgres() {
		return "pgx"
	}
	return "mysql"
}

// DSN builds the driver's DSN for this config (SQLDriver) - the single
// place every binary connecting to the site database uses.
func (c DBConfig) DSN() string {
	if c.IsPostgres() {
		return c.postgresDSN()
	}
	// clientFoundRows: an UPDATE reports the rows it matched, not only the
	// ones whose values changed - as on PostgreSQL.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&clientFoundRows=true", c.Username, c.Password, c.Host, c.Port, c.Scheme)
	if tls := strings.TrimSpace(c.TLS); tls != "" {
		dsn += "&tls=" + tls
	}
	if mode := strings.TrimSpace(c.SQLMode); mode != "" && mode != SQLModeServerDefault {
		// Unknown DSN params are sent by the driver as SET <name>=<value>
		// on connect; the value must be a quoted SQL string.
		dsn += "&sql_mode=" + url.QueryEscape("'"+mode+"'")
	}
	return dsn
}

// postgresDSN - a postgres:// URL; TLS is the sslmode.
func (c DBConfig) postgresDSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.Username, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   "/" + c.Scheme,
	}
	if mode := strings.TrimSpace(c.TLS); mode != "" {
		u.RawQuery = url.Values{"sslmode": {mode}}.Encode()
	}
	return u.String()
}

// SessionConfig - the session cookie: SecretKey - the key signing it
// (required with env "prod"; empty elsewhere - a random one per process),
// Secure - sent over HTTPS only, MaxAge - its lifetime in seconds.
type SessionConfig struct {
	SecretKey string `yaml:"secret_key" env:"SESSION_SECRET_KEY"`
	Secure    bool   `yaml:"secure" env:"SESSION_SECURE" env-default:"false"`
	MaxAge    int    `yaml:"max_age" env-default:"604800"` // 7 days in seconds
}

// TimeoutConfig - how long things may take, in seconds (yaml "timeouts").
type TimeoutConfig struct {
	// ServerRead / ServerWrite / ServerIdle - the HTTP server's read,
	// write (the whole response) and keep-alive idle timeouts.
	ServerRead  int `yaml:"server_read" env:"TIMEOUT_SERVER_READ" env-default:"15"`
	ServerWrite int `yaml:"server_write" env:"TIMEOUT_SERVER_WRITE" env-default:"15"`
	ServerIdle  int `yaml:"server_idle" env:"TIMEOUT_SERVER_IDLE" env-default:"60"`
	// Shutdown - how long a stopping server waits for requests in flight.
	Shutdown int `yaml:"shutdown" env:"TIMEOUT_SHUTDOWN" env-default:"30"`
	// DatabaseQuery - a page's database work (BaseController.DBContext).
	DatabaseQuery int `yaml:"database_query" env:"TIMEOUT_DATABASE_QUERY" env-default:"30"`
	// CacheUpdate - one refresh of the in-memory caches (sidebar blocks…).
	CacheUpdate int `yaml:"cache_update" env:"TIMEOUT_CACHE_UPDATE" env-default:"15"`
	// HTTPRequest - calls to outside services (reCAPTCHA, Resend, the SMTP
	// server).
	HTTPRequest int `yaml:"http_request" env:"TIMEOUT_HTTP_REQUEST" env-default:"30"`
}

// Seconds - n seconds as a duration (a timeout setting).
func Seconds(n int) time.Duration { return time.Duration(n) * time.Second }

// Load - the config at -config (before a command) or CONFIG_PATH.
func Load() (*Config, error) {
	configPath := fetchConfigPath()
	if configPath == "" {
		return nil, errors.New("no config: give -config <file> or CONFIG_PATH")
	}
	return LoadPath(configPath)
}

// LoadPath - the config at configPath, with the environment's values over
// the file's and the defaults that depend on the others.
func LoadPath(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("the config file %s does not exist", configPath)
	}
	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("reading the config %s: %w", configPath, err)
	}
	if err := applyEnv(&cfg); err != nil {
		return nil, err
	}
	applyDBTLSDefault(&cfg)
	applyDBPortDefault(&cfg)
	applyCatalogsOff(&cfg)
	return &cfg, nil
}

// applyCatalogsOff - "off" as a catalog's address is no catalog (an
// empty value in the file means the default one).
func applyCatalogsOff(cfg *Config) {
	for _, url := range []*string{&cfg.ThemesCatalogURL, &cfg.TranslationsCatalogURL} {
		if strings.EqualFold(strings.TrimSpace(*url), "off") {
			*url = ""
		}
	}
}

// MustLoadPath - LoadPath, a panic on a failure (the tests).
func MustLoadPath(configPath string) *Config {
	cfg, err := LoadPath(configPath)
	if err != nil {
		panic(err.Error())
	}
	return cfg
}

// IsLocal - the site runs for development (env local).
func (c *Config) IsLocal() bool { return c.Env == EnvLocal }

// IsProd - a live site: every env but local (LoadPath accepts no other).
func (c *Config) IsProd() bool { return !c.IsLocal() }

// applyEnv - env in its canonical form (any case, spaces around); a value
// other than local or prod is an error, not a mode of its own: a typo
// would turn the live site's checks off.
func applyEnv(cfg *Config) error {
	cfg.Env = strings.ToLower(strings.TrimSpace(cfg.Env))
	if cfg.Env != EnvLocal && cfg.Env != EnvProd {
		return fmt.Errorf("env %q (APP_ENV): %s or %s", cfg.Env, EnvProd, EnvLocal)
	}
	return nil
}

// applyDBPortDefault - the driver's usual port when db.port / DB_PORT is
// unset: 3306 for MySQL, 5432 for PostgreSQL.
func applyDBPortDefault(cfg *Config) {
	if cfg.DB.Port != 0 {
		return
	}
	cfg.DB.Port = 3306
	if cfg.DB.IsPostgres() {
		cfg.DB.Port = 5432
	}
}

// applyDBTLSDefault enables TLS for a remote database when db.tls / DB_TLS is unset.
// Many managed MySQL hosts use require_secure_transport=ON; without tls= in the DSN,
// Ping fails with ER_CLIENT_INTERACTION_WITHOUT_TLS (3159). PostgreSQL gets
// sslmode=require (encrypted, the certificate not verified - like skip-verify).
func applyDBTLSDefault(cfg *Config) {
	if strings.TrimSpace(cfg.DB.TLS) != "" {
		return
	}
	if cfg.IsLocal() {
		return
	}
	host := strings.TrimSpace(cfg.DB.Host)
	if host == "" || host == "localhost" || host == "127.0.0.1" {
		return
	}
	if cfg.DB.IsPostgres() {
		cfg.DB.TLS = "require"
		return
	}
	cfg.DB.TLS = "skip-verify"
}

// fetchConfigPath fetches config path from command line flag or environment variable.
// Priority: flag > env > default.
// Default value is empty string.
func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
