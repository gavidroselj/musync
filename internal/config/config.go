package config

import (
	"log/slog"
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

const (
	workDir    = "musync"
	configFile = "config.yml"
	stateFile  = "state.json"
)

type Config struct {
	LogLevel  slog.Level      `yaml:"log_level"`
	Playlists []PlaylistEntry `yaml:"playlists"`
}

type PlaylistEntry struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Path string `yaml:"path"`
}

func LoadConfig() (*Config, error) {
	filePath := ConfigFilePath()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return &Config{}, err
	}

	conf := Config{}
	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		return &Config{}, err
	}

	return &conf, nil
}

func ConfigFilePath() string {
	return path.Join(workDir, configFile)
}

func StateFilePath() string {
	return path.Join(workDir, stateFile)
}
