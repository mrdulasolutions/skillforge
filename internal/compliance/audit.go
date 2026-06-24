// Package compliance provides the opt-in audit + guardrail layer: an
// HMAC-chained append-only audit log and an untrusted-input sanitizer, ported
// from the ExChek MCP server patterns. The audit log lives inside each skill
// (.skillforge/audit.jsonl) for provenance; the signing key lives in the user
// config dir so it never travels with a packaged skill.
package compliance

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// appendMu serializes the read-prev-hmac + append critical section so concurrent
// Append calls in one process can't fork the chain.
var appendMu sync.Mutex

// Event is a request to record an audit entry.
type Event struct {
	EventType string
	Skill     string
	Tool      string
	Actor     string
	Summary   string
	Metadata  map[string]any
}

// entry is the canonical chained record (field order is significant for HMAC).
type entry struct {
	TS        string         `json:"ts"`
	PrevHMAC  *string        `json:"prev_hmac"`
	EventType string         `json:"event_type"`
	Skill     *string        `json:"skill"`
	Tool      *string        `json:"tool"`
	Actor     string         `json:"actor"`
	Summary   *string        `json:"summary"`
	Metadata  map[string]any `json:"metadata"`
}

type storedEntry struct {
	entry
	HMAC string `json:"hmac"`
}

// AppendResult is returned from Append.
type AppendResult struct {
	OK   bool
	HMAC string
	TS   string
}

// VerifyResult reports the outcome of a chain verification.
type VerifyResult struct {
	OK       bool
	Lines    int
	BrokenAt int // -1 when OK
	Reason   string
}

func auditDir(skillDir string) string { return filepath.Join(skillDir, ".skillforge") }
func logPath(skillDir string) string  { return filepath.Join(auditDir(skillDir), "audit.jsonl") }

// keyPath returns the global signing-key location (never inside a skill).
func keyPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "skillforge", "audit.key"), nil
}

func getKey() ([]byte, error) {
	if env := os.Getenv("SKILLFORGE_AUDIT_KEY"); len(env) >= 32 {
		return []byte(env), nil
	}
	kp, err := keyPath()
	if err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(kp); err == nil {
		return b, nil
	}
	if err := os.MkdirAll(filepath.Dir(kp), 0o700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(kp, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// getExistingKey reads the signing key without creating one (used by Verify, so
// a verify can't pollute the keystore or "verify" against a fresh wrong key).
func getExistingKey() ([]byte, bool) {
	if env := os.Getenv("SKILLFORGE_AUDIT_KEY"); len(env) >= 32 {
		return []byte(env), true
	}
	kp, err := keyPath()
	if err != nil {
		return nil, false
	}
	if b, err := os.ReadFile(kp); err == nil {
		return b, true
	}
	return nil, false
}

// decodeStored decodes a log line, keeping JSON numbers exact (UseNumber) so the
// HMAC re-marshal matches what was signed even for large integers in metadata.
func decodeStored(line string) (storedEntry, error) {
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	var s storedEntry
	err := dec.Decode(&s)
	return s, err
}

func computeHMAC(key []byte, e entry) (string, error) {
	payload, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	m := hmac.New(sha256.New, key)
	m.Write(payload)
	return hex.EncodeToString(m.Sum(nil)), nil
}

func lastHMAC(skillDir string) *string {
	b, err := os.ReadFile(logPath(skillDir))
	if err != nil {
		return nil
	}
	lines := nonEmptyLines(string(b))
	if len(lines) == 0 {
		return nil
	}
	var s storedEntry
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &s); err != nil {
		return nil
	}
	if s.HMAC == "" {
		return nil
	}
	return &s.HMAC
}

// Append writes a new HMAC-chained entry to the skill's audit log.
func Append(skillDir string, ev Event) (*AppendResult, error) {
	appendMu.Lock()
	defer appendMu.Unlock()
	if err := os.MkdirAll(auditDir(skillDir), 0o700); err != nil {
		return nil, err
	}
	key, err := getKey()
	if err != nil {
		return nil, err
	}
	e := entry{
		TS:        time.Now().UTC().Format(time.RFC3339),
		PrevHMAC:  lastHMAC(skillDir),
		EventType: orDefault(ev.EventType, "unspecified"),
		Skill:     ptrOrNil(ev.Skill),
		Tool:      ptrOrNil(ev.Tool),
		Actor:     orDefault(ev.Actor, "skillforge"),
		Summary:   ptrOrNil(clip(ev.Summary, 500)),
		Metadata:  ev.Metadata,
	}
	h, err := computeHMAC(key, e)
	if err != nil {
		return nil, err
	}
	line, err := json.Marshal(storedEntry{entry: e, HMAC: h})
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(logPath(skillDir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return nil, err
	}
	return &AppendResult{OK: true, HMAC: h, TS: e.TS}, nil
}

// Verify walks the chain and reports the first break, if any.
func Verify(skillDir string) (*VerifyResult, error) {
	b, err := os.ReadFile(logPath(skillDir))
	if err != nil {
		return &VerifyResult{OK: true, Lines: 0, BrokenAt: -1}, nil
	}
	lines := nonEmptyLines(string(b))
	key, ok := getExistingKey()
	if !ok {
		return &VerifyResult{OK: false, Lines: len(lines), BrokenAt: -1, Reason: "key_unavailable"}, nil
	}
	var prev *string
	for i, ln := range lines {
		s, err := decodeStored(ln)
		if err != nil {
			return &VerifyResult{OK: false, Lines: len(lines), BrokenAt: i, Reason: "parse_error"}, nil
		}
		if !ptrEq(s.PrevHMAC, prev) {
			return &VerifyResult{OK: false, Lines: len(lines), BrokenAt: i, Reason: "prev_hmac_mismatch"}, nil
		}
		expected, err := computeHMAC(key, s.entry)
		if err != nil {
			return nil, err
		}
		if expected != s.HMAC {
			return &VerifyResult{OK: false, Lines: len(lines), BrokenAt: i, Reason: "hmac_mismatch"}, nil
		}
		h := s.HMAC
		prev = &h
	}
	return &VerifyResult{OK: true, Lines: len(lines), BrokenAt: -1}, nil
}

// HasLog reports whether a skill has an audit log.
func HasLog(skillDir string) bool {
	_, err := os.Stat(logPath(skillDir))
	return err == nil
}

// Init creates the audit log for a freshly scaffolded skill.
func Init(skillDir, skillName string) error {
	_, err := Append(skillDir, Event{
		EventType: "scaffold",
		Skill:     skillName,
		Tool:      "skillforge new",
		Summary:   "compliance profile enabled; skill scaffolded",
	})
	return err
}

// --- small helpers ---

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func ptrEq(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
