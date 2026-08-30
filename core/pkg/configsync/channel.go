package configsync

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ChannelField describes one credential/config field of a storage channel.
type ChannelField struct {
	Name     string // env suffix, e.g. "APP_ID" → CENTAG_CONFIGSYNC_<CHANNEL>_APP_ID
	Prompt   string // human prompt (wizard)
	Secret   bool   // masked in logs; never persisted outside 0600 dotenv
	Required bool
}

// ChannelDescriptor describes a pluggable storage channel. Tools and the
// wizard are channel-agnostic shells over this registry — adding a channel
// means registering a descriptor plus a Provider implementation; the
// framework needs no changes.
type ChannelDescriptor struct {
	ID          string // "feishu", "snapshot", ...
	Description string
	Fields      []ChannelField
	// Validate performs a live API check with the given values. Returning
	// an error blocks persistence (TC-CFG-005).
	Validate func(ctx context.Context, values map[string]string) error
}

var channelRegistry = map[string]ChannelDescriptor{}

// RegisterChannel adds a channel descriptor. Later registrations with the
// same ID replace earlier ones.
func RegisterChannel(d ChannelDescriptor) { channelRegistry[d.ID] = d }

// ListChannels returns registered channel IDs in stable order.
func ListChannels() []string {
	ids := make([]string, 0, len(channelRegistry))
	for id := range channelRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// GetChannel looks up a descriptor by ID.
func GetChannel(id string) (ChannelDescriptor, bool) {
	d, ok := channelRegistry[id]
	return d, ok
}

// EnvName builds the channel-scoped env var name for a field:
// CENTAG_CONFIGSYNC_<CHANNEL>_<FIELD> (TC-CFG-003).
func EnvName(channel, field string) string {
	return "CENTAG_CONFIGSYNC_" + strings.ToUpper(channel) + "_" + strings.ToUpper(field)
}

// LoadOptions controls channel credential resolution.
type LoadOptions struct {
	Channel    string
	ConfigFile string // ① explicit --config path (highest priority)
	DefaultDir string // ③ default dotenv dir, e.g. config/secrets/configsync
	NoSave     bool   // skip persisting wizard results
	// Wizard is the interactive resolver (④). It receives the descriptor and
	// the values gathered so far, and returns the final values. nil = device
	// side: missing required fields are a hard error, never interactive
	// (TC-CFG-009).
	Wizard func(ctx context.Context, desc ChannelDescriptor, current map[string]string) (map[string]string, error)
}

// LoadChannelConfig resolves channel credentials with per-field priority:
// ① --config file → ② env → ③ default dotenv file → ④ wizard. The returned
// source map records where each field came from (for audits/tests).
func LoadChannelConfig(ctx context.Context, opts LoadOptions) (values map[string]string, sources map[string]string, err error) {
	desc, ok := GetChannel(opts.Channel)
	if !ok {
		return nil, nil, fmt.Errorf("configsync: unknown channel %q (registered: %v)", opts.Channel, ListChannels())
	}
	values = map[string]string{}
	sources = map[string]string{}
	// ③ default file is loaded first so ①/② can override per-field.
	if opts.DefaultDir != "" {
		path := filepath.Join(opts.DefaultDir, opts.Channel+".env")
		if fileVals, err := parseDotenvFile(path); err == nil {
			for k, v := range fileVals {
				values[k] = v
				sources[k] = "file:" + path
			}
		}
	}
	// ② environment overrides the default file.
	for _, f := range desc.Fields {
		if v, ok := os.LookupEnv(EnvName(opts.Channel, f.Name)); ok && strings.TrimSpace(v) != "" {
			values[f.Name] = strings.TrimSpace(v)
			sources[f.Name] = "env"
		}
	}
	// ① explicit --config file wins over everything.
	if opts.ConfigFile != "" {
		fileVals, err := parseDotenvFile(opts.ConfigFile)
		if err != nil {
			return nil, nil, fmt.Errorf("configsync: read --config: %w", err)
		}
		for k, v := range fileVals {
			values[k] = v
			sources[k] = "config:" + opts.ConfigFile
		}
	}
	// Required-field check; ④ wizard is the last resort.
	missing := missingRequired(desc, values)
	if len(missing) > 0 && opts.Wizard != nil {
		wizardVals, err := opts.Wizard(ctx, desc, values)
		if err != nil {
			return nil, nil, err
		}
		for k, v := range wizardVals {
			if strings.TrimSpace(v) != "" {
				values[k] = v
				if _, tracked := sources[k]; !tracked {
					sources[k] = "wizard"
				}
			}
		}
		missing = missingRequired(desc, values)
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("configsync: channel %q missing required fields: %s", opts.Channel, strings.Join(missing, ", "))
	}
	// Persist wizard results unless suppressed (TC-CFG-006).
	if opts.Wizard != nil && !opts.NoSave && opts.DefaultDir != "" {
		if err := SaveChannelConfig(opts.Channel, values, opts.DefaultDir); err != nil {
			return nil, nil, err
		}
	}
	return values, sources, nil
}

func missingRequired(desc ChannelDescriptor, values map[string]string) []string {
	var missing []string
	for _, f := range desc.Fields {
		if f.Required && strings.TrimSpace(values[f.Name]) == "" {
			missing = append(missing, f.Name)
		}
	}
	return missing
}

// SaveChannelConfig writes values as dotenv to <dir>/<channel>.env with 0600.
func SaveChannelConfig(channel string, values map[string]string, dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("configsync: mkdir secrets dir: %w", err)
	}
	path := filepath.Join(dir, channel+".env")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("configsync: write %s: %w", path, err)
	}
	defer f.Close()
	// Stable order for reviewable diffs.
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(f, "%s=%s\n", k, values[k]); err != nil {
			return err
		}
	}
	return nil
}

func parseDotenvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseDotenv(f), nil
}

func parseDotenv(r io.Reader) map[string]string {
	vals := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vals[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return vals
}

// RunWizardCore is the channel-agnostic wizard: it asks every field via the
// injected ask function (pre-filled with current values), then validates via
// the descriptor hook. On validation failure nothing is persisted — the
// caller simply receives the error (TC-CFG-005).
func RunWizardCore(ctx context.Context, desc ChannelDescriptor, current map[string]string, ask func(field ChannelField, current string) (string, error)) (map[string]string, error) {
	out := map[string]string{}
	for _, f := range desc.Fields {
		cur := current[f.Name]
		v, err := ask(f, cur)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(v) != "" || (cur != "" && v == cur) {
			out[f.Name] = strings.TrimSpace(v)
			if out[f.Name] == "" {
				out[f.Name] = cur
			}
		}
	}
	if desc.Validate != nil {
		if err := desc.Validate(ctx, out); err != nil {
			return nil, fmt.Errorf("configsync: channel %q validation failed: %w", desc.ID, err)
		}
	}
	return out, nil
}

// ResolveTableID resolves a table ID by name when not explicitly configured,
// using the base's table list (TC-CFG-007 免配项).
type tableLister interface {
	FindTable(ctx context.Context, appToken, name string) (string, error)
}

func ResolveTableID(ctx context.Context, c tableLister, appToken, tableID, tableName string) (string, error) {
	if strings.TrimSpace(tableID) != "" {
		return tableID, nil
	}
	return c.FindTable(ctx, appToken, tableName)
}
