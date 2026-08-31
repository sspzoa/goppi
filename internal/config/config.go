package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sspzoa/goppi/internal/upstage"
)

var Version = "0.168.0"

var Efforts = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

type Config struct {
	APIKey          string               `json:"api_key,omitempty"`
	BaseURL         string               `json:"base_url,omitempty"`
	Provider        string               `json:"provider,omitempty"`
	Model           string               `json:"model"`
	Mode            string               `json:"mode,omitempty"`
	ReasoningEffort string               `json:"reasoning_effort,omitempty"`
	MaxTurns        int                  `json:"max_turns"`
	WorkDir         string               `json:"workdir"`
	MaxTokens       int                  `json:"max_tokens,omitempty"`
	AutoCompact     bool                 `json:"auto_compact"`
	CompactAt       int                  `json:"compact_at,omitempty"`
	PromptCacheKey  string               `json:"prompt_cache_key,omitempty"`
	AlwaysApprove   bool                 `json:"always_approve,omitempty"`
	Sandbox         string               `json:"sandbox,omitempty"`
	MCPServers      map[string]MCPServer `json:"mcp_servers,omitempty"`
	LSPServers      map[string]LSPServer `json:"lsp_servers,omitempty"`
	Worktree        bool                 `json:"worktree,omitempty"`
	Hooks           Hooks                `json:"hooks,omitempty"`
	OutputFormat    string               `json:"-"`
	ExtraDirs       []string             `json:"-"`
}

type Hooks struct {
	PreTool      []Hook `json:"pre_tool,omitempty"`
	PostTool     []Hook `json:"post_tool,omitempty"`
	SessionStart []Hook `json:"session_start,omitempty"`
	SessionEnd   []Hook `json:"session_end,omitempty"`
}

type Hook struct {
	Matcher string `json:"matcher,omitempty"`
	Command string `json:"command"`
}

type MCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type LSPServer struct {
	Command  string   `json:"command"`
	Args     []string `json:"args,omitempty"`
	Language string   `json:"language,omitempty"`
}

func Default() Config {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return Config{
		Provider:        "upstage",
		BaseURL:         upstage.DefaultBaseURL,
		Model:           upstage.DefaultModel,
		Mode:            "act",
		ReasoningEffort: "medium",
		Sandbox:         "workspace",
		MaxTurns:        30,
		WorkDir:         wd,
		MaxTokens:       32768,
		AutoCompact:     true,
		CompactAt:       100000,
	}
}

func Load() (Config, error) {
	cfg := Default()
	if home, err := os.UserHomeDir(); err == nil {
		if err := mergeFile(&cfg, filepath.Join(home, ".config", "goppi", "config.json"), true); err != nil {
			return cfg, err
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if err := mergeFile(&cfg, filepath.Join(wd, ".goppi.json"), false); err != nil {
			return cfg, err
		}
	}
	applyEnv(&cfg)
	if err := cfg.Normalize(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// mergeFile overlays JSON onto cfg. Untrusted project files cannot set
// always_approve, api_key, sandbox, mcp_servers, lsp_servers, worktree, or hooks — those stay on the user config, env, or CLI.
func mergeFile(cfg *Config, path string, trusted bool) error {
	if trusted {
		if err := SecretLinkError(path); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config %s: %w", path, err)
	}
	unknown, err := unknownConfigKeys(data)
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if len(unknown) > 0 {
		return fmt.Errorf("config %s: unknown key %s", path, strings.Join(unknown, ", "))
	}
	prevKey, prevYolo, prevMCP, prevLSP, prevSandbox, prevWT, prevHooks := cfg.APIKey, cfg.AlwaysApprove, cloneMCP(cfg.MCPServers), cloneLSP(cfg.LSPServers), cfg.Sandbox, cfg.Worktree, cloneHooks(cfg.Hooks)
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if !trusted {
		cfg.APIKey = prevKey
		cfg.AlwaysApprove = prevYolo
		cfg.MCPServers = prevMCP
		cfg.LSPServers = prevLSP
		cfg.Sandbox = prevSandbox
		cfg.Worktree = prevWT
		cfg.Hooks = prevHooks
	}
	return nil
}

var knownConfigKeys = map[string]bool{
	"api_key": true, "base_url": true, "provider": true, "model": true,
	"mode": true, "reasoning_effort": true, "max_turns": true, "workdir": true,
	"max_tokens": true, "auto_compact": true, "compact_at": true,
	"prompt_cache_key": true, "always_approve": true, "sandbox": true,
	"mcp_servers": true, "lsp_servers": true, "worktree": true, "hooks": true,
}

func unknownConfigKeys(data []byte) ([]string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var bad []string
	for k := range raw {
		if !knownConfigKeys[k] {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad, nil
}

func cloneMCP(in map[string]MCPServer) map[string]MCPServer {
	if in == nil {
		return nil
	}
	out := make(map[string]MCPServer, len(in))
	for k, v := range in {
		if v.Args != nil {
			v.Args = append([]string(nil), v.Args...)
		}
		if v.Env != nil {
			env := make(map[string]string, len(v.Env))
			for ek, ev := range v.Env {
				env[ek] = ev
			}
			v.Env = env
		}
		out[k] = v
	}
	return out
}

func cloneHooks(in Hooks) Hooks {
	return Hooks{
		PreTool:      cloneHookList(in.PreTool),
		PostTool:     cloneHookList(in.PostTool),
		SessionStart: cloneHookList(in.SessionStart),
		SessionEnd:   cloneHookList(in.SessionEnd),
	}
}

func cloneHookList(in []Hook) []Hook {
	if len(in) == 0 {
		return nil
	}
	return append([]Hook(nil), in...)
}

func cloneLSP(in map[string]LSPServer) map[string]LSPServer {
	if in == nil {
		return nil
	}
	out := make(map[string]LSPServer, len(in))
	for k, v := range in {
		if v.Args != nil {
			v.Args = append([]string(nil), v.Args...)
		}
		out[k] = v
	}
	return out
}

func (c Config) LSPNames() []string {
	if len(c.LSPServers) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.LSPServers))
	for name := range c.LSPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c Config) MCPNames() []string {
	if len(c.MCPServers) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.MCPServers))
	for name := range c.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) Normalize() error {
	return normalize(c)
}

func (c Config) ResolveAPIKey() string {
	if k := upstage.ResolveAPIKey(c.APIKey); k != "" {
		return k
	}
	return LoadStoredAPIKey()
}

func (c Config) KeySource() string {
	if strings.TrimSpace(c.APIKey) != "" {
		return "config.json"
	}
	if os.Getenv("UPSTAGE_API_KEY") != "" {
		return "UPSTAGE_API_KEY"
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return "OPENAI_API_KEY"
	}
	if os.Getenv("GOPPI_API_KEY") != "" {
		return "GOPPI_API_KEY"
	}
	if LoadStoredAPIKey() != "" {
		return "goppi login"
	}
	return ""
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("GOPPI_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("UPSTAGE_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("GOPPI_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("GOPPI_WORKDIR"); v != "" {
		cfg.WorkDir = v
	}
	if v := os.Getenv("GOPPI_EFFORT"); v != "" {
		cfg.ReasoningEffort = v
	}
	if v := os.Getenv("UPSTAGE_REASONING_EFFORT"); v != "" {
		cfg.ReasoningEffort = v
	}
	if envTruthy(os.Getenv("GOPPI_ALWAYS_APPROVE")) {
		cfg.AlwaysApprove = true
	}
	if v := os.Getenv("GOPPI_PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("GOPPI_MODE"); v != "" {
		cfg.Mode = v
	}
	if v := os.Getenv("GOPPI_SANDBOX"); v != "" {
		cfg.Sandbox = v
	}
	if envTruthy(os.Getenv("GOPPI_WORKTREE")) {
		cfg.Worktree = true
	}
	if v := os.Getenv("GOPPI_AUTO_COMPACT"); v != "" {
		switch {
		case envTruthy(v):
			cfg.AutoCompact = true
		case envFalsy(v):
			cfg.AutoCompact = false
		}
	}
	if v := strings.TrimSpace(os.Getenv("GOPPI_COMPACT_AT")); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			cfg.CompactAt = n
		}
	}
}

func envFalsy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off":
		return true
	default:
		return false
	}
}

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalize(cfg *Config) error {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if cfg.Provider == "" {
		cfg.Provider = "upstage"
	}
	switch cfg.Provider {
	case "upstage", "openai", "compat", "openai-compat":
	default:
		return fmt.Errorf("unknown provider %q (upstage|openai|compat)", cfg.Provider)
	}
	if cfg.Provider == "openai-compat" {
		cfg.Provider = "compat"
	}
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode == "" {
		cfg.Mode = "act"
	}
	if cfg.Mode != "act" && cfg.Mode != "plan" {
		return fmt.Errorf("unknown mode %q (act|plan)", cfg.Mode)
	}
	if cfg.BaseURL == "" {
		if cfg.Provider == "openai" {
			cfg.BaseURL = "https://api.openai.com/v1"
		} else {
			cfg.BaseURL = upstage.DefaultBaseURL
		}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 30
	}
	if cfg.MaxTurns > 80 {
		cfg.MaxTurns = 80
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 32768
	}
	if cfg.MaxTokens > 131072 {
		cfg.MaxTokens = 131072
	}
	if cfg.CompactAt <= 0 {
		cfg.CompactAt = 100000
	}
	if cfg.CompactAt < 8000 {
		cfg.CompactAt = 8000
	}
	if cfg.CompactAt > 500000 {
		cfg.CompactAt = 500000
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = "."
	}
	abs, err := normalizeDir(cfg.WorkDir, "workdir")
	if err != nil {
		return err
	}
	cfg.WorkDir = abs
	if extras, err := normalizeExtraDirs(cfg.WorkDir, cfg.ExtraDirs); err != nil {
		return err
	} else {
		cfg.ExtraDirs = extras
	}
	if cfg.Model == "" {
		cfg.Model = upstage.DefaultModel
	}
	if !AllowsModel(*cfg) {
		return fmt.Errorf("unknown model %q", cfg.Model)
	}
	cfg.ReasoningEffort = strings.ToLower(strings.TrimSpace(cfg.ReasoningEffort))
	if cfg.ReasoningEffort != "" && !validEffort(cfg.ReasoningEffort) {
		return fmt.Errorf("unknown reasoning_effort %q (%s)", cfg.ReasoningEffort, strings.Join(Efforts, "|"))
	}
	if cfg.Model == "solar-mini" {
		cfg.ReasoningEffort = ""
	} else if cfg.ReasoningEffort == "" {
		// solar-pro4 docs say omit == on, but with tools+stream the API
		// often skips reasoning unless effort is sent explicitly.
		cfg.ReasoningEffort = "medium"
	}
	sandbox, err := NormalizeSandbox(cfg.Sandbox)
	if err != nil {
		return err
	}
	cfg.Sandbox = sandbox
	cfg.Hooks = normalizeHooks(cfg.Hooks)
	return nil
}

const maxHooksPerEvent = 8

func normalizeHooks(h Hooks) Hooks {
	return Hooks{
		PreTool:      clipHooks(h.PreTool),
		PostTool:     clipHooks(h.PostTool),
		SessionStart: clipHooks(h.SessionStart),
		SessionEnd:   clipHooks(h.SessionEnd),
	}
}

func clipHooks(in []Hook) []Hook {
	var out []Hook
	for _, h := range in {
		h.Command = strings.TrimSpace(h.Command)
		h.Matcher = strings.TrimSpace(h.Matcher)
		if h.Command == "" {
			continue
		}
		if len(h.Command) > 4096 {
			h.Command = h.Command[:4096]
		}
		out = append(out, h)
		if len(out) >= maxHooksPerEvent {
			break
		}
	}
	return out
}

func (c Config) HookCounts() map[string]int {
	return map[string]int{
		"pre_tool":      len(c.Hooks.PreTool),
		"post_tool":     len(c.Hooks.PostTool),
		"session_start": len(c.Hooks.SessionStart),
		"session_end":   len(c.Hooks.SessionEnd),
	}
}

func NormalizeSandbox(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "workspace":
		return "workspace", nil
	case "strict":
		return "strict", nil
	case "off":
		return "off", nil
	default:
		return "", fmt.Errorf("unknown sandbox %q (workspace|strict|off)", s)
	}
}

func AllowsModel(cfg Config) bool {
	if upstage.KnownModel(cfg.Model) {
		return true
	}
	if !cfg.Compat() {
		return false
	}
	m := strings.TrimSpace(cfg.Model)
	if m == "" || strings.ContainsAny(m, " \t\n") {
		return false
	}
	return true
}

func (c Config) Compat() bool {
	switch c.Provider {
	case "openai", "compat":
		return true
	default:
		return false
	}
}

func (c Config) DocumentTools() bool {
	return c.Provider == "upstage" || strings.Contains(c.BaseURL, "upstage.ai")
}

func validEffort(s string) bool {
	for _, e := range Efforts {
		if e == s {
			return true
		}
	}
	return false
}

func UserConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "goppi")
	return dir, ensureRealDir(dir)
}

func UserDataDir() (string, error) {
	if v := os.Getenv("GOPPI_DATA_DIR"); v != "" {
		return v, ensureRealDir(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "goppi")
	return dir, ensureRealDir(dir)
}

func ensureRealDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return SecretLinkError(dir)
}

func EnsureDataSubdir(name string) (string, error) {
	root, err := UserDataDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(root, name)
	return d, ensureRealDir(d)
}

func ProbeWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".goppi-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_, err = f.Write([]byte("ok"))
	_ = f.Close()
	_ = os.Remove(name)
	return err
}

func credentialsPath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

func CredentialsPath() (string, error) {
	return credentialsPath()
}

// SecretPermError is nil when path is missing or owner-only
// (no group/other bits). Existing world/group-readable key files
// are the leak doctor must catch.
func SecretEnvKey(key string) bool {
	u := strings.ToUpper(key)
	switch {
	case strings.Contains(u, "API_KEY"), strings.Contains(u, "SECRET"), strings.HasSuffix(u, "_TOKEN"):
		return true
	case u == "AWS_SECRET_ACCESS_KEY", u == "AWS_ACCESS_KEY_ID":
		return true
	default:
		return false
	}
}

func ScrubEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if SecretEnvKey(key) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func SecretLinkError(path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink", path)
	}
	return nil
}

func SecretPermError(path string, want os.FileMode) error {
	if err := SecretLinkError(path); err != nil {
		return err
	}
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	perm := st.Mode().Perm()
	if perm&0o077 != 0 {
		return fmt.Errorf("%s is mode %04o (want %04o)", path, perm, want)
	}
	return nil
}

func TightenSecret(path string, mode os.FileMode) error {
	if err := SecretLinkError(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

func UserConfigPath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func FileHasAPIKey(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var file struct {
		APIKey string `json:"api_key"`
	}
	if json.Unmarshal(data, &file) != nil {
		return false
	}
	return strings.TrimSpace(file.APIKey) != ""
}

func WorldWritableError(path string) error {
	if err := SecretLinkError(path); err != nil {
		return err
	}
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if st.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("%s is world-writable (%04o)", path, st.Mode().Perm())
	}
	return nil
}

func LoadStoredAPIKey() string {
	path, err := credentialsPath()
	if err != nil {
		return ""
	}
	if SecretLinkError(path) != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var file struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return ""
	}
	return strings.TrimSpace(file.APIKey)
}

func SaveAPIKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("empty API key")
	}
	if len(key) > 2048 {
		return fmt.Errorf("API key too long")
	}
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		APIKey string `json:"api_key"`
	}{APIKey: key}, "", "  ")
	if err != nil {
		return err
	}
	return writeSecret(path, data)
}

func writeSecret(path string, data []byte) error {
	if st, err := os.Lstat(path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	dir := filepath.Dir(path)
	if err := ensureRealDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".goppi-secret-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(name, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

const maxExtraDirs = 8

func normalizeDir(p, kind string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("%s: empty path", kind)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("%s: %w", kind, err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("%s: %w", kind, err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%s is not a directory", kind)
	}
	check := abs
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		check = real
	}
	if isFSRoot(check) {
		return "", fmt.Errorf("%s cannot be the filesystem root", kind)
	}
	return abs, nil
}

func normalizeExtraDirs(primary string, dirs []string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, nil
	}
	seen := map[string]bool{}
	if real, err := filepath.EvalSymlinks(primary); err == nil {
		seen[real] = true
	} else {
		seen[primary] = true
	}
	var out []string
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if !filepath.IsAbs(d) {
			return nil, fmt.Errorf("additional directory must be an absolute path")
		}
		abs, err := normalizeDir(d, "additional directory")
		if err != nil {
			return nil, err
		}
		key := abs
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			key = real
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(out) >= maxExtraDirs {
			return nil, fmt.Errorf("additional directories: more than %d", maxExtraDirs)
		}
		out = append(out, abs)
	}
	return out, nil
}

func isFSRoot(p string) bool {
	p = filepath.Clean(p)
	if p == string(os.PathSeparator) || p == "/" {
		return true
	}
	vol := filepath.VolumeName(p)
	if vol == "" {
		return false
	}
	rest := strings.Trim(strings.TrimPrefix(p, vol), `/\`)
	return rest == ""
}

func ClearAPIKey() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
