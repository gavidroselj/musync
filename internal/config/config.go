package config

import (
	"log/slog"
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

const (
	WorkDir    = "config"
	configFile = "config.yml"
	stateFile  = "state.json"
)

type Config struct {
	LogLevel     slog.Level      `yaml:"log_level"`
	Playlists    []PlaylistEntry `yaml:"playlists"`
	Destinations Destinations    `yaml:"destinations"`
}

type Destinations struct {
	LocalDestinations  []LocalDestination  `yaml:"local"`
	WebdavDestinations []WebdavDestination `yaml:"webdav"`
}

type PlaylistEntry struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Path        string `yaml:"path"`
	Destination string `yaml:"destination"`
}

type LocalDestination struct {
	Name string `yaml:"name"`
	Root string `yaml:"root"`
}

type WebdavDestination struct {
	Name     string `yaml:"name"`
	RootURL  string `yaml:"root_url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
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
	return path.Join(WorkDir, configFile)
}

func StateFilePath() string {
	return path.Join(WorkDir, stateFile)
}
