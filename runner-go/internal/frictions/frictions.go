// Package frictions records in an append-only JSONL file the friction an
// orchestrator meets while working: a tool that answers invalid, a check the
// environment blocked, a return in prose. It aggregates on read; it measures
// and never fixes. The file is the user's own, outside the runner store.
package frictions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Classes and Surfaces are closed sets: a value outside them is rejected.
var (
	Classes  = []string{"contract", "tool", "environment", "doc"}
	Surfaces = []string{"runner", "tasks-ai", "acm", "opencode", "subagent", "git"}
)

// FixResult says what Fix found for a fingerprint.
type FixResult int

const (
	Fixed        FixResult = iota // the entry was open and is now closed
	AlreadyFixed                  // the entry existed and was already closed
	UnknownFP                     // the fingerprint does not exist
)

const layout = time.RFC3339

var digits = regexp.MustCompile(`[0-9]+`)

// Event is one line of the file: an add or a fixed marker.
type Event struct {
	T        string `json:"t"`
	FP       string `json:"fp"`
	Class    string `json:"class,omitempty"`
	Surface  string `json:"surface,omitempty"`
	Key      string `json:"key,omitempty"`
	Observed string `json:"observed,omitempty"`
	Project  string `json:"project,omitempty"`
	Fixed    bool   `json:"fixed,omitempty"`
}

// Entry is the aggregation of every event of one fingerprint.
type Entry struct {
	FP       string   `json:"fp"`
	Class    string   `json:"class"`
	Surface  string   `json:"surface"`
	Key      string   `json:"key"`
	Count    int      `json:"count"`
	First    string   `json:"first"`
	Last     string   `json:"last"`
	State    string   `json:"state"`
	Projects []string `json:"projects"`
	Observed string   `json:"observed"`

	order int // file position of the last add, the tie-break of the list
}

// Record is the input of one add event. Now injects the clock so tests are
// deterministic.
type Record struct {
	Class    string
	Surface  string
	Key      string
	Observed string
	Project  string
	Now      time.Time
}

// Normalize lowercases, collapses whitespace, turns every digit run into # and
// trims spaces and trailing punctuation.
func Normalize(text string) string {
	normalized := strings.Join(strings.Fields(text), " ")
	normalized = strings.ToLower(normalized)
	normalized = digits.ReplaceAllString(normalized, "#")
	return strings.TrimRight(normalized, " .,;:")
}

// Fingerprint is the identity of a friction: the first 12 hex characters of
// sha256(surface + "\n" + class + "\n" + key).
func Fingerprint(surface, class, key string) string {
	sum := sha256.Sum256([]byte(surface + "\n" + class + "\n" + key))
	return hex.EncodeToString(sum[:])[:12]
}

// ValidClass and ValidSurface report membership in the closed sets.
func ValidClass(class string) bool     { return contains(Classes, class) }
func ValidSurface(surface string) bool { return contains(Surfaces, surface) }

// Path resolves the registry: ORCHESTRATOR_FRICTIONS or
// <UserConfigDir>/opencode/frictions.jsonl.
func Path() (string, error) {
	if path := os.Getenv("ORCHESTRATOR_FRICTIONS"); path != "" {
		return path, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "opencode", "frictions.jsonl"), nil
}

// Read parses the file in order. It returns the readable events and how many
// lines it had to skip.
func Read(path string) ([]Event, int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	var events []Event
	skipped := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event Event
		if json.Unmarshal([]byte(line), &event) != nil || event.FP == "" {
			skipped++
			continue
		}
		if _, err := time.Parse(layout, event.T); err != nil {
			skipped++
			continue
		}
		events = append(events, event)
	}
	return events, skipped, nil
}

// Aggregate groups the events in file order. Every add counts and reopens; a
// fixed closes the entry; a later add reopens it.
func Aggregate(events []Event) []Entry {
	entries := make([]Entry, 0)
	index := map[string]int{}
	for i, event := range events {
		j, seen := index[event.FP]
		if event.Fixed {
			if seen {
				entries[j].State = "fixed"
			}
			continue
		}
		if !seen {
			index[event.FP] = len(entries)
			entries = append(entries, Entry{
				FP: event.FP, Class: event.Class, Surface: event.Surface,
				Key: event.Key, Count: 1, First: event.T, Last: event.T,
				State: "open", Projects: []string{}, Observed: event.Observed,
				order: i,
			})
			if event.Project != "" {
				entries[len(entries)-1].Projects = []string{event.Project}
			}
			continue
		}
		entry := &entries[j]
		entry.Count++
		entry.Last = event.T
		entry.State = "open"
		entry.Observed = event.Observed
		entry.order = i
		if event.Project != "" && !contains(entry.Projects, event.Project) {
			entry.Projects = append(entry.Projects, event.Project)
		}
	}
	return entries
}

// Select filters by minimum count and caps the result; limit 0 means all. It
// sorts by count and, on a tie, by the most recent occurrence.
func Select(entries []Entry, min, limit int) []Entry {
	sorted := append([]Entry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].order > sorted[j].order
	})
	out := make([]Entry, 0, len(sorted))
	for _, entry := range sorted {
		if entry.Count < min {
			continue
		}
		if limit > 0 && len(out) >= limit {
			break
		}
		out = append(out, entry)
	}
	return out
}

// Open keeps the entries still open: a fixed friction leaves the list and stays
// in the file as history.
func Open(entries []Entry) []Entry {
	open := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.State != "fixed" {
			open = append(open, entry)
		}
	}
	return open
}

// Add appends one add event and returns the entry after the write. The key
// defaults to Normalize(Observed).
func Add(path string, record Record) (Entry, error) {
	key := strings.TrimSpace(record.Key)
	if key == "" {
		key = Normalize(record.Observed)
	}
	fp := Fingerprint(record.Surface, record.Class, key)
	event := Event{
		T: record.Now.UTC().Format(layout), FP: fp,
		Class: record.Class, Surface: record.Surface, Key: key,
		Observed: record.Observed, Project: record.Project,
	}
	if err := appendLine(path, event); err != nil {
		return Entry{}, err
	}
	events, _, err := Read(path)
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range Aggregate(events) {
		if entry.FP == fp {
			return entry, nil
		}
	}
	return Entry{}, fmt.Errorf("la fricción %s no aparece tras escribirla", fp)
}

// Fix closes the entry of fp and reports whether it was open, already closed or
// unknown.
func Fix(path, fp string, now time.Time) (FixResult, error) {
	events, _, err := Read(path)
	if err != nil {
		return UnknownFP, err
	}
	for _, entry := range Aggregate(events) {
		if entry.FP != fp {
			continue
		}
		if entry.State == "fixed" {
			return AlreadyFixed, nil
		}
		return Fixed, appendLine(path, Event{T: now.UTC().Format(layout), FP: fp, Fixed: true})
	}
	return UnknownFP, nil
}

func appendLine(path string, event Event) error {
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// shortcut: la concurrencia se apoya en la atomicidad de la escritura única con O_APPEND, pon un lock si aparece corrupción.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
