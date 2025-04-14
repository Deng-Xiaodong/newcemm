package config

type Config struct {
	ClientCnt  int `json:"client_cnt"`
	Volume     int `json:"volume"`
	LimitRound int `json:"limit"`
}

var defaultConfig = Config{
	ClientCnt:  50,
	Volume:     1_0,
	LimitRound: 2_000,
}

func GetDefaultConfig() *Config {
	return &defaultConfig
}
