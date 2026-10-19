// Package config 负责加载 aide 的运行配置。
//
// 加载优先级（后者覆盖前者）：
//
//	内置默认值  <  配置文件(YAML/JSON)  <  环境变量(AIDE_*)
//
// 这样既能开箱即用，又能在不改代码的前提下切换模型、服务地址与工作目录。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是 aide 的完整运行配置。
type Config struct {
	// ServerURL 是 OpenAI 兼容服务的基础地址（不含 /chat/completions）。
	ServerURL string `json:"server_url" yaml:"server_url"`
	// Model 是通用对话使用的模型名。
	Model string `json:"model" yaml:"model"`
	// CoderModel 是 explain/commit 等编码场景使用的模型名。
	CoderModel string `json:"coder_model" yaml:"coder_model"`
	// APIKey 为可选字段；本地 Ollama 无需填写，使用云端时通过环境变量注入。
	APIKey string `json:"api_key" yaml:"api_key"`
	// WorkDir 是只读工具允许访问的根目录（工具不得越出该目录）。
	WorkDir string `json:"work_dir" yaml:"work_dir"`
	// HTTPAddr 是 `aide serve` 的监听地址，默认仅本机回环。
	HTTPAddr string `json:"http_addr" yaml:"http_addr"`
	// Timeout 是单次非流式请求的超时时间。
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// MaxFileBytes 是 read_file 允许读取的单文件字节上限。
	MaxFileBytes int64 `json:"max_file_bytes" yaml:"max_file_bytes"`
	// MaxToolTurns 是一次请求中“模型调用工具”的最大轮数，防止失控循环。
	MaxToolTurns int `json:"max_tool_turns" yaml:"max_tool_turns"`
	// Temperature 是采样温度。
	Temperature float64 `json:"temperature" yaml:"temperature"`
	// Verbose 打开调试日志。
	Verbose bool `json:"verbose" yaml:"verbose"`
}

// fileConfig 是磁盘文件的映射结构。
// time.Duration 无法直接从 "2m" 这类字符串解析，因此这里用字符串承接，
// 解析后再转换为 Config.Timeout，保证配置文件对人类友好。
type fileConfig struct {
	ServerURL    string  `json:"server_url" yaml:"server_url"`
	Model        string  `json:"model" yaml:"model"`
	CoderModel   string  `json:"coder_model" yaml:"coder_model"`
	APIKey       string  `json:"api_key" yaml:"api_key"`
	WorkDir      string  `json:"work_dir" yaml:"work_dir"`
	HTTPAddr     string  `json:"http_addr" yaml:"http_addr"`
	Timeout      string  `json:"timeout" yaml:"timeout"`
	MaxFileBytes int64   `json:"max_file_bytes" yaml:"max_file_bytes"`
	MaxToolTurns int     `json:"max_tool_turns" yaml:"max_tool_turns"`
	Temperature  float64 `json:"temperature" yaml:"temperature"`
	Verbose      bool    `json:"verbose" yaml:"verbose"`
}

// Default 返回内置默认配置（面向本机 Ollama 开箱即用）。
func Default() *Config {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return &Config{
		ServerURL:    "http://127.0.0.1:11434/v1",
		Model:        "qwen3.6:35b-mlx",
		CoderModel:   "qwen3-coder-next:latest",
		WorkDir:      wd,
		HTTPAddr:     "127.0.0.1:8080",
		Timeout:      120 * time.Second,
		MaxFileBytes: 256 * 1024,
		MaxToolTurns: 6,
		Temperature:  0.7,
	}
}

// Load 按 默认值 -> 文件 -> 环境变量 的顺序加载配置并校验。
// path 为空时，自动尝试当前目录下的 config.yaml / config.yml。
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		for _, candidate := range []string{"config.yaml", "config.yml"} {
			if fileExists(candidate) {
				path = candidate
				break
			}
		}
	}
	if path != "" {
		if err := loadFile(cfg, path); err != nil {
			return nil, err
		}
	}

	applyEnv(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate 校验配置并对 WorkDir 做绝对路径规范化。
func (c *Config) Validate() error {
	if strings.TrimSpace(c.ServerURL) == "" {
		return fmt.Errorf("server_url 不能为空")
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("model 不能为空")
	}
	if c.MaxFileBytes <= 0 {
		return fmt.Errorf("max_file_bytes 必须为正数，当前=%d", c.MaxFileBytes)
	}
	if c.MaxToolTurns <= 0 {
		return fmt.Errorf("max_tool_turns 必须为正数，当前=%d", c.MaxToolTurns)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout 必须为正数，当前=%s", c.Timeout)
	}
	abs, err := filepath.Abs(c.WorkDir)
	if err != nil {
		return fmt.Errorf("work_dir 无法解析为绝对路径: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("work_dir 不可访问: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("work_dir 不是目录: %s", abs)
	}
	c.WorkDir = abs
	c.ServerURL = strings.TrimRight(c.ServerURL, "/")
	return nil
}

// loadFile 根据扩展名选择 YAML 或 JSON 解析，并把结果合并进 cfg（仅覆盖非零值）。
func loadFile(cfg *Config, path string) error {
	// path 来自用户显式指定的配置文件，G304 在此是预期行为。
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return fmt.Errorf("读取配置文件失败 %s: %w", path, err)
	}
	var fc fileConfig
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return fmt.Errorf("解析 YAML 失败: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &fc); err != nil {
			return fmt.Errorf("解析 JSON 失败: %w", err)
		}
	default:
		return fmt.Errorf("不支持的配置文件类型: %s（仅支持 .yaml/.yml/.json）", ext)
	}
	mergeFileConfig(cfg, &fc)
	return nil
}

// mergeFileConfig 把文件中的非零值覆盖到 cfg。
func mergeFileConfig(cfg *Config, fc *fileConfig) {
	if fc.ServerURL != "" {
		cfg.ServerURL = fc.ServerURL
	}
	if fc.Model != "" {
		cfg.Model = fc.Model
	}
	if fc.CoderModel != "" {
		cfg.CoderModel = fc.CoderModel
	}
	if fc.APIKey != "" {
		cfg.APIKey = fc.APIKey
	}
	if fc.WorkDir != "" {
		cfg.WorkDir = fc.WorkDir
	}
	if fc.HTTPAddr != "" {
		cfg.HTTPAddr = fc.HTTPAddr
	}
	if fc.Timeout != "" {
		if d, err := time.ParseDuration(fc.Timeout); err == nil {
			cfg.Timeout = d
		}
	}
	if fc.MaxFileBytes != 0 {
		cfg.MaxFileBytes = fc.MaxFileBytes
	}
	if fc.MaxToolTurns != 0 {
		cfg.MaxToolTurns = fc.MaxToolTurns
	}
	if fc.Temperature != 0 {
		cfg.Temperature = fc.Temperature
	}
	cfg.Verbose = fc.Verbose
}

// applyEnv 用 AIDE_* 环境变量覆盖配置（环境变量优先级最高）。
func applyEnv(cfg *Config) {
	if v := os.Getenv("AIDE_SERVER_URL"); v != "" {
		cfg.ServerURL = v
	}
	if v := os.Getenv("AIDE_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("AIDE_CODER_MODEL"); v != "" {
		cfg.CoderModel = v
	}
	if v := os.Getenv("AIDE_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("AIDE_WORK_DIR"); v != "" {
		cfg.WorkDir = v
	}
	if v := os.Getenv("AIDE_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("AIDE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeout = d
		}
	}
	if v := os.Getenv("AIDE_MAX_FILE_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxFileBytes = n
		}
	}
	if v := os.Getenv("AIDE_MAX_TOOL_TURNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxToolTurns = n
		}
	}
	if v := os.Getenv("AIDE_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Temperature = f
		}
	}
	if v := os.Getenv("AIDE_VERBOSE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Verbose = b
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
