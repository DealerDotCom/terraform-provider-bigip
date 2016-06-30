// Copyright 2014 Unknwon
// Copyright 2014 Torkel Ödegaard

package setting

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/go-macaron/session"
	"gopkg.in/ini.v1"

	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/util"
)

type Scheme string

const (
	HTTP  Scheme = "http"
	HTTPS Scheme = "https"
)

const (
	DEV  string = "development"
	PROD string = "production"
	TEST string = "test"
)

var (
	// App settings.
	Env          string = DEV
	AppUrl       string
	AppSubUrl    string
	InstanceName string

	// build
	BuildVersion string
	BuildCommit  string
	BuildStamp   int64

	// Paths
	LogsPath    string
	HomePath    string
	DataPath    string
	PluginsPath string

	// Log settings.
	LogModes   []string
	LogConfigs []util.DynMap

	// Http server options
	Protocol           Scheme
	Domain             string
	HttpAddr, HttpPort string
	SshPort            int
	CertFile, KeyFile  string
	RouterLogging      bool
	StaticRootPath     string
	EnableGzip         bool
	EnforceDomain      bool

	// Security settings.
	SecretKey             string
	LogInRememberDays     int
	CookieUserName        string
	CookieRememberName    string
	DisableGravatar       bool
	EmailCodeValidMinutes int
	DataProxyWhiteList    map[string]bool

	// Snapshots
	ExternalSnapshotUrl  string
	ExternalSnapshotName string
	ExternalEnabled      bool

	// User settings
	AllowUserSignUp    bool
	AllowUserOrgCreate bool
	AutoAssignOrg      bool
	AutoAssignOrgRole  string
	VerifyEmailEnabled bool
	LoginHint          string
	DefaultTheme       string
	AllowUserPassLogin bool

	// Http auth
	AdminUser     string
	AdminPassword string

	AnonymousEnabled bool
	AnonymousOrgName string
	AnonymousOrgRole string

	// Auth proxy settings
	AuthProxyEnabled        bool
	AuthProxyHeaderName     string
	AuthProxyHeaderProperty string
	AuthProxyAutoSignUp     bool

	// Basic Auth
	BasicAuthEnabled bool

	// Session settings.
	SessionOptions session.Options

	// Global setting objects.
	Cfg          *ini.File
	ConfRootPath string
	IsWindows    bool

	// PhantomJs Rendering
	ImagesDir  string
	PhantomDir string

	// for logging purposes
	configFiles                  []string
	appliedCommandLineProperties []string
	appliedEnvOverrides          []string

	ReportingEnabled   bool
	CheckForUpdates    bool
	GoogleAnalyticsId  string
	GoogleTagManagerId string

	// LDAP
	LdapEnabled    bool
	LdapConfigFile string

	// SMTP email settings
	Smtp SmtpSettings

	// QUOTA
	Quota QuotaSettings

	// logger
	logger log.Logger

	// Grafana.NET URL
	GrafanaNetUrl string
)

type CommandLineArgs struct {
	Config   string
	HomePath string
	Args     []string
}

func init() {
	IsWindows = runtime.GOOS == "windows"
	logger = log.New("settings")
}

func parseAppUrlAndSubUrl(section *ini.Section) (string, string) {
	appUrl := section.Key("root_url").MustString("http://localhost:3000/")
	if appUrl[len(appUrl)-1] != '/' {
		appUrl += "/"
	}

	// Check if has app suburl.
	url, err := url.Parse(appUrl)
	if err != nil {
		log.Fatal(4, "Invalid root_url(%s): %s", appUrl, err)
	}
	appSubUrl := strings.TrimSuffix(url.Path, "/")

	return appUrl, appSubUrl
}

func ToAbsUrl(relativeUrl string) string {
	return AppUrl + relativeUrl
}

func shouldRedactKey(s string) bool {
	uppercased := strings.ToUpper(s)
	return strings.Contains(uppercased, "PASSWORD") || strings.Contains(uppercased, "SECRET")
}

func applyEnvVariableOverrides() {
	appliedEnvOverrides = make([]string, 0)
	for _, section := range Cfg.Sections() {
		for _, key := range section.Keys() {
			sectionName := strings.ToUpper(strings.Replace(section.Name(), ".", "_", -1))
			keyName := strings.ToUpper(strings.Replace(key.Name(), ".", "_", -1))
			envKey := fmt.Sprintf("GF_%s_%s", sectionName, keyName)
			envValue := os.Getenv(envKey)

			if len(envValue) > 0 {
				key.SetValue(envValue)
				if shouldRedactKey(envKey) {
					envValue = "*********"
				}
				appliedEnvOverrides = append(appliedEnvOverrides, fmt.Sprintf("%s=%s", envKey, envValue))
			}
		}
	}
}

func applyCommandLineDefaultProperties(props map[string]string) {
	appliedCommandLineProperties = make([]string, 0)
	for _, section := range Cfg.Sections() {
		for _, key := range section.Keys() {
			keyString := fmt.Sprintf("default.%s.%s", section.Name(), key.Name())
			value, exists := props[keyString]
			if exists {
				key.SetValue(value)
				if shouldRedactKey(keyString) {
					value = "*********"
				}
				appliedCommandLineProperties = append(appliedCommandLineProperties, fmt.Sprintf("%s=%s", keyString, value))
			}
		}
	}
}

func applyCommandLineProperties(props map[string]string) {
	for _, section := range Cfg.Sections() {
		for _, key := range section.Keys() {
			keyString := fmt.Sprintf("%s.%s", section.Name(), key.Name())
			value, exists := props[keyString]
			if exists {
				key.SetValue(value)
				appliedCommandLineProperties = append(appliedCommandLineProperties, fmt.Sprintf("%s=%s", keyString, value))
			}
		}
	}
}

func getCommandLineProperties(args []string) map[string]string {
	props := make(map[string]string)

	for _, arg := range args {
		if !strings.HasPrefix(arg, "cfg:") {
			continue
		}

		trimmed := strings.TrimPrefix(arg, "cfg:")
		parts := strings.Split(trimmed, "=")
		if len(parts) != 2 {
			log.Fatal(3, "Invalid command line argument", arg)
			return nil
		}

		props[parts[0]] = parts[1]
	}
	return props
}

func makeAbsolute(path string, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func evalEnvVarExpression(value string) string {
	regex := regexp.MustCompile(`\${(\w+)}`)
	return regex.ReplaceAllStringFunc(value, func(envVar string) string {
		envVar = strings.TrimPrefix(envVar, "${")
		envVar = strings.TrimSuffix(envVar, "}")
		envValue := os.Getenv(envVar)

		// if env variable is hostname and it is emtpy use os.Hostname as default
		if envVar == "HOSTNAME" && envValue == "" {
			envValue, _ = os.Hostname()
		}

		return envValue
	})
}

func evalConfigValues() {
	for _, section := range Cfg.Sections() {
		for _, key := range section.Keys() {
			key.SetValue(evalEnvVarExpression(key.Value()))
		}
	}
}

func loadSpecifedConfigFile(configFile string) {
	if configFile == "" {
		configFile = filepath.Join(HomePath, "conf/custom.ini")
		// return without error if custom file does not exist
		if !pathExists(configFile) {
			return
		}
	}

	userConfig, err := ini.Load(configFile)
	userConfig.BlockMode = false
	if err != nil {
		log.Fatal(3, "Failed to parse %v, %v", configFile, err)
	}

	for _, section := range userConfig.Sections() {
		for _, key := range section.Keys() {
			if key.Value() == "" {
				continue
			}

			defaultSec, err := Cfg.GetSection(section.Name())
			if err != nil {
				defaultSec, _ = Cfg.NewSection(section.Name())
			}
			defaultKey, err := defaultSec.GetKey(key.Name())
			if err != nil {
				defaultKey, _ = defaultSec.NewKey(key.Name(), key.Value())
			}
			defaultKey.SetValue(key.Value())
		}
	}

	configFiles = append(configFiles, configFile)
}

func loadConfiguration(args *CommandLineArgs) {
	var err error

	// load config defaults
	defaultConfigFile := path.Join(HomePath, "conf/defaults.ini")
	configFiles = append(configFiles, defaultConfigFile)

	Cfg, err = ini.Load(defaultConfigFile)
	Cfg.BlockMode = false

	if err != nil {
		log.Fatal(3, "Failed to parse defaults.ini, %v", err)
	}

	// command line props
	commandLineProps := getCommandLineProperties(args.Args)
	// load default overrides
	applyCommandLineDefaultProperties(commandLineProps)

	// init logging before specific config so we can log errors from here on
	DataPath = makeAbsolute(Cfg.Section("paths").Key("data").String(), HomePath)
	initLogging()

	// load specified config file
	loadSpecifedConfigFile(args.Config)

	// apply environment overrides
	applyEnvVariableOverrides()

	// apply command line overrides
	applyCommandLineProperties(commandLineProps)

	// evaluate config values containing environment variables
	evalConfigValues()

	// update data path and logging config
	DataPath = makeAbsolute(Cfg.Section("paths").Key("data").String(), HomePath)
	initLogging()
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func setHomePath(args *CommandLineArgs) {
	if args.HomePath != "" {
		HomePath = args.HomePath
		return
	}

	HomePath, _ = filepath.Abs(".")
	// check if homepath is correct
	if pathExists(filepath.Join(HomePath, "conf/defaults.ini")) {
		return
	}

	// try down one path
	if pathExists(filepath.Join(HomePath, "../conf/defaults.ini")) {
		HomePath = filepath.Join(HomePath, "../")
	}
}

var skipStaticRootValidation bool = false

func validateStaticRootPath() error {
	if skipStaticRootValidation {
		return nil
	}

	if _, err := os.Stat(path.Join(StaticRootPath, "css")); err == nil {
		return nil
	}

	if _, err := os.Stat(StaticRootPath + "_gen/css"); err == nil {
		StaticRootPath = StaticRootPath + "_gen"
		return nil
	}

	return fmt.Errorf("Failed to detect generated css or javascript files in static root (%s), have you executed default grunt task?", StaticRootPath)
}

// func readInstanceName() string {
// 	hostname, _ := os.Hostname()
// 	if hostname == "" {
// 		hostname = "hostname_unknown"
// 	}
//
// 	instanceName := Cfg.Section("").Key("instance_name").MustString("")
// 	if instanceName = "" {
// 		// set value as it might be used in other places
// 		Cfg.Section("").Key("instance_name").SetValue(hostname)
// 		instanceName = hostname
// 	}
//
// 	return
// }

func NewConfigContext(args *CommandLineArgs) error {
	setHomePath(args)
	loadConfiguration(args)

	Env = Cfg.Section("").Key("app_mode").MustString("development")
	InstanceName = Cfg.Section("").Key("instance_name").MustString("unknown_instance_name")
	PluginsPath = Cfg.Section("paths").Key("plugins").String()

	server := Cfg.Section("server")
	AppUrl, AppSubUrl = parseAppUrlAndSubUrl(server)

	Protocol = HTTP
	if server.Key("protocol").MustString("http") == "https" {
		Protocol = HTTPS
		CertFile = server.Key("cert_file").String()
		KeyFile = server.Key("cert_key").String()
	}

	Domain = server.Key("domain").MustString("localhost")
	HttpAddr = server.Key("http_addr").MustString("0.0.0.0")
	HttpPort = server.Key("http_port").MustString("3000")
	RouterLogging = server.Key("router_logging").MustBool(false)
	EnableGzip = server.Key("enable_gzip").MustBool(false)
	EnforceDomain = server.Key("enforce_domain").MustBool(false)
	StaticRootPath = makeAbsolute(server.Key("static_root_path").String(), HomePath)

	if err := validateStaticRootPath(); err != nil {
		return err
	}

	// read security settings
	security := Cfg.Section("security")
	SecretKey = security.Key("secret_key").String()
	LogInRememberDays = security.Key("login_remember_days").MustInt()
	CookieUserName = security.Key("cookie_username").String()
	CookieRememberName = security.Key("cookie_remember_name").String()
	DisableGravatar = security.Key("disable_gravatar").MustBool(true)

	// read snapshots settings
	snapshots := Cfg.Section("snapshots")
	ExternalSnapshotUrl = snapshots.Key("external_snapshot_url").String()
	ExternalSnapshotName = snapshots.Key("external_snapshot_name").String()
	ExternalEnabled = snapshots.Key("external_enabled").MustBool(true)

	//  read data source proxy white list
	DataProxyWhiteList = make(map[string]bool)
	for _, hostAndIp := range security.Key("data_source_proxy_whitelist").Strings(" ") {
		DataProxyWhiteList[hostAndIp] = true
	}

	// admin
	AdminUser = security.Key("admin_user").String()
	AdminPassword = security.Key("admin_password").String()

	users := Cfg.Section("users")
	AllowUserSignUp = users.Key("allow_sign_up").MustBool(true)
	AllowUserOrgCreate = users.Key("allow_org_create").MustBool(true)
	AutoAssignOrg = users.Key("auto_assign_org").MustBool(true)
	AutoAssignOrgRole = users.Key("auto_assign_org_role").In("Editor", []string{"Editor", "Admin", "Read Only Editor", "Viewer"})
	VerifyEmailEnabled = users.Key("verify_email_enabled").MustBool(false)
	LoginHint = users.Key("login_hint").String()
	DefaultTheme = users.Key("default_theme").String()
	AllowUserPassLogin = users.Key("allow_user_pass_login").MustBool(true)

	// anonymous access
	AnonymousEnabled = Cfg.Section("auth.anonymous").Key("enabled").MustBool(false)
	AnonymousOrgName = Cfg.Section("auth.anonymous").Key("org_name").String()
	AnonymousOrgRole = Cfg.Section("auth.anonymous").Key("org_role").String()

	// auth proxy
	authProxy := Cfg.Section("auth.proxy")
	AuthProxyEnabled = authProxy.Key("enabled").MustBool(false)
	AuthProxyHeaderName = authProxy.Key("header_name").String()
	AuthProxyHeaderProperty = authProxy.Key("header_property").String()
	AuthProxyAutoSignUp = authProxy.Key("auto_sign_up").MustBool(true)

	authBasic := Cfg.Section("auth.basic")
	BasicAuthEnabled = authBasic.Key("enabled").MustBool(true)

	// PhantomJS rendering
	ImagesDir = filepath.Join(DataPath, "png")
	PhantomDir = filepath.Join(HomePath, "vendor/phantomjs")

	analytics := Cfg.Section("analytics")
	ReportingEnabled = analytics.Key("reporting_enabled").MustBool(true)
	CheckForUpdates = analytics.Key("check_for_updates").MustBool(true)
	GoogleAnalyticsId = analytics.Key("google_analytics_ua_id").String()
	GoogleTagManagerId = analytics.Key("google_tag_manager_id").String()

	ldapSec := Cfg.Section("auth.ldap")
	LdapEnabled = ldapSec.Key("enabled").MustBool(false)
	LdapConfigFile = ldapSec.Key("config_file").String()

	readSessionConfig()
	readSmtpSettings()
	readQuotaSettings()

	if VerifyEmailEnabled && !Smtp.Enabled {
		log.Warn("require_email_validation is enabled but smpt is disabled")
	}

	GrafanaNetUrl = Cfg.Section("grafana.net").Key("url").MustString("https://grafana.net")

	return nil
}

func readSessionConfig() {
	sec := Cfg.Section("session")
	SessionOptions = session.Options{}
	SessionOptions.Provider = sec.Key("provider").In("memory", []string{"memory", "file", "redis", "mysql", "postgres", "memcache"})
	SessionOptions.ProviderConfig = strings.Trim(sec.Key("provider_config").String(), "\" ")
	SessionOptions.CookieName = sec.Key("cookie_name").MustString("grafana_sess")
	SessionOptions.CookiePath = AppSubUrl
	SessionOptions.Secure = sec.Key("cookie_secure").MustBool()
	SessionOptions.Gclifetime = Cfg.Section("session").Key("gc_interval_time").MustInt64(86400)
	SessionOptions.Maxlifetime = Cfg.Section("session").Key("session_life_time").MustInt64(86400)
	SessionOptions.IDLength = 16

	if SessionOptions.Provider == "file" {
		SessionOptions.ProviderConfig = makeAbsolute(SessionOptions.ProviderConfig, DataPath)
		os.MkdirAll(path.Dir(SessionOptions.ProviderConfig), os.ModePerm)
	}

	if SessionOptions.CookiePath == "" {
		SessionOptions.CookiePath = "/"
	}
}

func initLogging() {
	// split on comma
	LogModes = strings.Split(Cfg.Section("log").Key("mode").MustString("console"), ",")
	// also try space
	if len(LogModes) == 1 {
		LogModes = strings.Split(Cfg.Section("log").Key("mode").MustString("console"), " ")
	}
	LogsPath = makeAbsolute(Cfg.Section("paths").Key("logs").String(), HomePath)
	log.ReadLoggingConfig(LogModes, LogsPath, Cfg)
}

func LogConfigurationInfo() {
	var text bytes.Buffer

	for _, file := range configFiles {
		logger.Info("Config loaded from", "file", file)
	}

	if len(appliedCommandLineProperties) > 0 {
		for _, prop := range appliedCommandLineProperties {
			logger.Info("Config overriden from command line", "arg", prop)
		}
	}

	if len(appliedEnvOverrides) > 0 {
		text.WriteString("\tEnvironment variables used:\n")
		for _, prop := range appliedEnvOverrides {
			logger.Info("Config overriden from Environment variable", "var", prop)
		}
	}

	logger.Info("Path Home", "path", HomePath)
	logger.Info("Path Data", "path", DataPath)
	logger.Info("Path Logs", "path", LogsPath)
	logger.Info("Path Plugins", "path", PluginsPath)
}
