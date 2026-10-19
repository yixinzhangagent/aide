package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.ServerURL == "" || c.Model == "" {
		t.Fatalf("默认配置缺少必要字段: %+v", c)
	}
	if c.MaxFileBytes <= 0 || c.MaxToolTurns <= 0 {
		t.Fatalf("默认数值字段应为正数: %+v", c)
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
server_url: "http://example/v1"
model: "test-model"
coder_model: "test-coder"
timeout: "30s"
max_file_bytes: 1024
max_tool_turns: 3
temperature: 0.2
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if c.ServerURL != "http://example/v1" {
		t.Errorf("ServerURL = %q", c.ServerURL)
	}
	if c.Model != "test-model" {
		t.Errorf("Model = %q", c.Model)
	}
	if c.CoderModel != "test-coder" {
		t.Errorf("CoderModel = %q", c.CoderModel)
	}
	if c.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", c.Timeout)
	}
	if c.MaxFileBytes != 1024 {
		t.Errorf("MaxFileBytes = %d", c.MaxFileBytes)
	}
	if c.MaxToolTurns != 3 {
		t.Errorf("MaxToolTurns = %d", c.MaxToolTurns)
	}
	if c.Temperature != 0.2 {
		t.Errorf("Temperature = %v", c.Temperature)
	}
}

func TestLoadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"server_url":"http://j/v1","model":"m","timeout":"5m"}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if c.ServerURL != "http://j/v1" {
		t.Errorf("ServerURL = %q", c.ServerURL)
	}
	if c.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v", c.Timeout)
	}
}

func TestEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AIDE_SERVER_URL", "http://env/v1")
	t.Setenv("AIDE_MODEL", "env-model")
	t.Setenv("AIDE_TIMEOUT", "10s")
	t.Setenv("AIDE_MAX_TOOL_TURNS", "9")
	t.Setenv("AIDE_VERBOSE", "true")

	c, err := Load(filepath.Join(dir, "absent.yaml"))
	// 显式指定不存在的文件应报错；这里改为不指定路径，测试 env 覆盖。
	_ = c
	_ = err

	c2, err := Load("")
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if c2.ServerURL != "http://env/v1" {
		t.Errorf("ServerURL = %q", c2.ServerURL)
	}
	if c2.Model != "env-model" {
		t.Errorf("Model = %q", c2.Model)
	}
	if c2.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v", c2.Timeout)
	}
	if c2.MaxToolTurns != 9 {
		t.Errorf("MaxToolTurns = %d", c2.MaxToolTurns)
	}
	if !c2.Verbose {
		t.Errorf("Verbose 应为 true")
	}
}

func TestMissingFileError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "no-such.yaml")); err == nil {
		t.Fatal("显式指定不存在的文件应当返回错误")
	}
}

func TestUnsupportedExt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("不支持的扩展名应当返回错误")
	}
}

func TestValidateErrors(t *testing.T) {
	c := Default()
	c.ServerURL = ""
	if err := c.Validate(); err == nil {
		t.Fatal("ServerURL 为空应报错")
	}

	c = Default()
	c.Model = ""
	if err := c.Validate(); err == nil {
		t.Fatal("Model 为空应报错")
	}

	c = Default()
	c.MaxFileBytes = 0
	if err := c.Validate(); err == nil {
		t.Fatal("MaxFileBytes<=0 应报错")
	}

	c = Default()
	c.WorkDir = filepath.Join(t.TempDir(), "no-dir")
	if err := c.Validate(); err == nil {
		t.Fatal("不存在的 WorkDir 应报错")
	}
}

func TestTrailingSlashTrimmed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`server_url: "http://x/v1/"`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.ServerURL != "http://x/v1" {
		t.Errorf("ServerURL 尾部斜杠应被去除，得到 %q", c.ServerURL)
	}
}
