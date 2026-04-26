// tools_redact.go implements the §5.2 URL redaction policy: strip
// userinfo, mask sensitive query keys (password/pwd/secret/token/code/otp).
// Returns (canonical, original); the canonical form is what gets logged
// + persisted + opened; original is the audit-only column.
package core

import (
	"net/url"
	"strings"
)

var sensitiveKeys = map[string]struct{}{
	"password": {},
	"pwd":      {},
	"secret":   {},
	"token":    {},
	"code":     {},
	"otp":      {},
}

// redactUrl returns (canonical, original). On parse failure, returns
// (raw, raw) so the caller can still launch unmodified.
func redactUrl(raw string) (string, string) {
	u, err := url.Parse(raw)
	if err != nil {
		return raw, raw
	}
	stripUserinfo(u)
	maskQuery(u)
	return u.String(), raw
}

func stripUserinfo(u *url.URL) {
	if u.User != nil {
		u.User = nil
	}
}

func maskQuery(u *url.URL) {
	if u.RawQuery == "" {
		return
	}
	q := u.Query()
	for k, vs := range q {
		if _, hit := sensitiveKeys[strings.ToLower(k)]; !hit {
			continue
		}
		for i := range vs {
			vs[i] = "***"
		}
		q[k] = vs
	}
	u.RawQuery = q.Encode()
}
