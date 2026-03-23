package executor

import (
	"encoding/json"
	"os"
)

type Config struct {
	DockerHost string                    `json:"docker_host"`
	Languages  map[Language]LangSettings `json:"languages"`
}

type LangSettings struct {
	Image       string   `json:"image"`
	SourceFile  string   `json:"source_file"`
	CompileCmd  []string `json:"compile_cmd,omitempty"`
	RunCmd      []string `json:"run_cmd"`
	MemoryLimit int64    `json:"memory_limit"`
	TimeLimit   int64    `json:"time_limit"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		Languages: map[Language]LangSettings{
			"python": {
				Image:      "python:3.14-slim",
				SourceFile: "main.py",
				RunCmd:     []string{"python", "main.py"},
			},
			"go": {
				Image:      "golang:1.26-alpine",
				SourceFile: "main.go",
				RunCmd:     []string{"go", "run", "main.go"},
			},
			"cpp": {
				Image:      "gcc:15",
				SourceFile: "main.cpp",
				CompileCmd: []string{"g++", "-O2", "-o", "solution", "main.cpp"},
				RunCmd:     []string{"./solution"},
			},
			"java": {
				Image:      "eclipse-temurin:21-jdk-alpine",
				SourceFile: "Main.java",
				CompileCmd: []string{"javac", "Main.java"},
				RunCmd:     []string{"java", "Main"},
			},
		},
	}
}
