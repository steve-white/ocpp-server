package config

type CacheConfig struct {
	HostPort string `mapstructure:"host_port"`
	Password string `mapstructure:"password"`
	DbId     int    `mapstructure:"db_id"`
}

type Configuration struct {
	Schema   string `mapstructure:"schema"`
	Services struct {
		CsmsServer struct {
			Debug          bool        `mapstructure:"debug"`
			EnableAuth     bool        `mapstructure:"enable_auth"`
			StandaloneMode bool        `mapstructure:"standalone_mode"`
			ListenAddress  string      `mapstructure:"listen_address"`
			ListenPort     int         `mapstructure:"listen_port"`
			Cache          CacheConfig `mapstructure:"cache"`
		} `mapstructure:"csms_server"`
		MessageManager struct {
			Debug              bool   `mapstructure:"debug"`
			StorageAccountName string `mapstructure:"storage_account_name"`
			StorageAccountKey  string `mapstructure:"storage_account_key"`
			StoreMessages      bool   `mapstructure:"store_messages"`
		} `mapstructure:"message_manager"`
		Session struct {
			Debug bool `mapstructure:"debug"`
		} `mapstructure:"session"`
		DeviceManager struct {
			Debug      bool       `mapstructure:"debug"`
			HttpConfig HttpConfig `mapstructure:"http_config"`
		} `mapstructure:"device_manager"`
	} `mapstructure:"services"`
	Logging struct {
		AppInsightsInstrumentationKey string `mapstructure:"appinsights_instrumentation_key"`
	}
	Mq       MqConfig `mapstructure:"mq"`
	DbConfig DbConfig `mapstructure:"db_config"`
}

type DbConfig struct {
	DbType             string `mapstructure:"type"`
	DbConnectionString string `mapstructure:"connection_string"`
}

type HttpConfig struct {
	ListenAddress string `mapstructure:"listen_address"`
	ListenPort    int    `mapstructure:"listen_port"`
	HttpUser      string `mapstructure:"http_user"`
	HttpPassword  string `mapstructure:"http_password"`
	TimeoutMs     int    `mapstructure:"timeoutms"`
	IdleTimeoutMs int    `mapstructure:"idle_timeoutms"`
}

type MqConfig struct {
	Type     string `mapstructure:"type"`
	MangosMq struct {
		CsmsListenUrl        string `mapstructure:"csms_listen_url"`
		CsmsListenRequestUrl string `mapstructure:"csms_listen_request_url"`
		SessionListenUrl     string `mapstructure:"session_listen_url"`
		MessageListenUrl     string `mapstructure:"message_listen_url"`
		DeviceListenUrl      string `mapstructure:"device_listen_url"`
	} `mapstructure:"mangos_mq"`
	RabbitMq struct {
		ServerUrl string `mapstructure:"server_url"`
	} `mapstructure:"rabbit_mq"`
	RedisMq CacheConfig `mapstructure:"redis_mq"`
}

type MqConnection struct {
	MqType            string `mapstructure:"mqType"`
	AmqpServerUrl     string `mapstructure:"amqpServerUrl"`
	MangosMqListenUrl string `mapstructure:"mangosMqListenUrl"`

	RedisHostPort string `mapstructure:"redisHostPort"`
	RedisPassword string `mapstructure:"redisPassword"`
	RedisDbId     int    `mapstructure:"redisDbId"`
}
