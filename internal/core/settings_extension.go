// settings_extension.go reads/writes the optional "settings" block in
// config.json without disturbing the existing camelCase shape that older
// versions of `email-read` understand. We keep this in its own file so the
// raw-IO surface can be replaced atomically once the M004 PascalCase
// migration lands.
package core

import (
	"encoding/json"
	"errors"
	"os"
	"sort"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// loadExtension parses just the "settings" key out of config.json. Missing
// file or missing key both yield a zero-value extension (no error).
func loadExtension() (settingsExtension, error) {
	p, err := config.Path()
	if err != nil {
		return settingsExtension{}, errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "extension path")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settingsExtension{}, nil
		}
		return settingsExtension{}, errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "read config for extension").
			WithContext("path", p)
	}
	var holder struct {
		Settings settingsExtension `json:"settings"`
	}
	if err := json.Unmarshal(b, &holder); err != nil {
		return settingsExtension{}, errtrace.WrapCode(err,
			errtrace.ErrConfigDecode, "decode extension").
			WithContext("path", p)
	}
	return holder.Settings, nil
}

// (saveExtension was inlined into settings.saveRaw to keep the typed
// config and the extension block in a single atomic write — no partial
// state is observable on disk.)


// readConfigAsMap reads the file as a generic JSON object. Missing file →
// empty map.
func readConfigAsMap(p string) (map[string]any, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, errtrace.WrapCode(err, errtrace.ErrConfigOpen,
			"read config map").WithContext("path", p)
	}
	root := map[string]any{}
	if len(b) == 0 {
		return root, nil
	}
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, errtrace.WrapCode(err, errtrace.ErrConfigDecode,
			"decode config map").WithContext("path", p)
	}
	return root, nil
}

// writeConfigMap re-serializes the map and writes via tmp+rename. Note:
// config.Save will run again separately to persist the typed Config — that
// second write overwrites this one for the typed fields, which is fine
// because we set the "settings" key from this side and the typed fields
// from the other.
func writeConfigMap(p string, root map[string]any) error {
	b, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return errtrace.WrapCode(err, errtrace.ErrConfigEncode,
			"encode config map")
	}
	tmp := p + ".settings.tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return errtrace.WrapCode(err, errtrace.ErrConfigEncode,
			"write extension tmp").WithContext("path", tmp)
	}
	if err := os.Rename(tmp, p); err != nil {
		return errtrace.WrapCode(err, errtrace.ErrConfigEncode,
			"rename extension tmp").WithContext("path", p)
	}
	return nil
}
