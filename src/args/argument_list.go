package args

import sdk_args "github.com/newrelic/infra-integrations-sdk/v3/args"

type ArgumentList struct {
	sdk_args.DefaultArgumentList
	Hostname                         string `default:"localhost" help:"Hostname or IP where MySQL is running."`
	Port                             int    `default:"3306" help:"Port on which MySQL server is listening."`
	Socket                           string `default:"" help:"MySQL Socket file."`
	Username                         string `default:"root" help:"Username for accessing the database."`
	Password                         string `default:"password" help:"Password for the given user."`
	Database                         string `default:"employees" help:"Database name"`
	ExtraConnectionURLArgs           string `help:"Specify extra connection parameters as attr1=val1&attr2=val2."` // https://github.com/go-sql-driver/mysql#parameters
	InsecureSkipVerify               bool   `default:"false" help:"Skip verification of the server's certificate when using TLS with the connection."`
	EnableTLS                        bool   `default:"false" help:"Use a secure (TLS) connection."`
	RemoteMonitoring                 bool   `default:"false" help:"Identifies the monitored entity as 'remote'. In doubt: set to true"`
	ExtendedMetrics                  bool   `default:"false" help:"Enable extended metrics"`
	ExtendedInnodbMetrics            bool   `default:"false" help:"Enable InnoDB extended metrics"`
	ExtendedMyIsamMetrics            bool   `default:"false" help:"Enable MyISAM extended metrics"`
	OldPasswords                     bool   `default:"false" help:"Allow old passwords: https://dev.mysql.com/doc/refman/5.6/en/server-system-variables.html#sysvar_old_passwords"`
	ShowVersion                      bool   `default:"false" help:"Print build information and exit"`
	EnableQueryPerformanceMonitoring bool   `default:"true" help:"Enable query performance monitoring"`
}
