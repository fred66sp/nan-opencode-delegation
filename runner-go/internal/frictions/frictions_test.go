package frictions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"  Input_Needed: falta EVIDENCE.  ": "input_needed: falta evidence",
		"checks no cubre 12 líneas...":      "checks no cubre # líneas",
		"Semántica  y\t espacios ;":         "semántica y espacios",
		"error 500: timeout":                "error #: timeout",
		"":                                  "",
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFingerprint(t *testing.T) {
	fp := Fingerprint("runner", "contract", "prueba:duplicado")
	if len(fp) != 12 {
		t.Fatalf("huella %q: %d caracteres, want 12", fp, len(fp))
	}
	for _, r := range fp {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("huella %q fuera del alfabeto hexadecimal", fp)
		}
	}
	if again := Fingerprint("runner", "contract", "prueba:duplicado"); again != fp {
		t.Errorf("huella inestable: %q != %q", again, fp)
	}
	for _, other := range []string{
		Fingerprint("runner", "tool", "prueba:duplicado"),
		Fingerprint("git", "contract", "prueba:duplicado"),
		Fingerprint("runner", "contract", "prueba:otra"),
	} {
		if other == fp {
			t.Errorf("la huella %q colisiona al cambiar clase, superficie o clave", other)
		}
	}
}

func TestAddCountsAndKeepsFirstAndLast(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frictions.jsonl")
	first := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	second := first.Add(3 * time.Minute)

	if _, err := Add(path, Record{
		Class: "contract", Surface: "runner", Key: "prueba:duplicado",
		Observed: "input_needed: falta evidence", Project: "demo", Now: first,
	}); err != nil {
		t.Fatal(err)
	}
	entry, err := Add(path, Record{
		Class: "contract", Surface: "runner", Key: "prueba:duplicado",
		Observed: "input_needed: falta otra", Project: "demo", Now: second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Count != 2 {
		t.Errorf("conteo = %d, want 2", entry.Count)
	}
	if entry.First != "2026-11-28T10:00:00Z" || entry.Last != "2026-11-28T10:03:00Z" {
		t.Errorf("first/last = %q/%q", entry.First, entry.Last)
	}
	if entry.Observed != "input_needed: falta otra" {
		t.Errorf("observed = %q, want la más reciente", entry.Observed)
	}
	if entry.State != "open" {
		t.Errorf("estado = %q, want open", entry.State)
	}
}

func TestFixedClosesAndAddReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frictions.jsonl")
	start := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	record := Record{
		Class: "contract", Surface: "runner", Key: "prueba:duplicado",
		Observed: "input_needed: falta evidence", Project: "demo", Now: start,
	}
	if _, err := Add(path, record); err != nil {
		t.Fatal(err)
	}
	fp := Fingerprint(record.Surface, record.Class, record.Key)

	if result, err := Fix(path, fp, "", start.Add(time.Minute)); err != nil || result != Fixed {
		t.Fatalf("Fix = %v, %v; want Fixed", result, err)
	}
	if entry := only(t, path); entry.State != "fixed" {
		t.Errorf("estado tras fixed = %q, want fixed", entry.State)
	}
	if result, err := Fix(path, fp, "", start.Add(2*time.Minute)); err != nil || result != AlreadyFixed {
		t.Fatalf("segundo Fix = %v, %v; want AlreadyFixed", result, err)
	}

	record.Now = start.Add(3 * time.Minute)
	if _, err := Add(path, record); err != nil {
		t.Fatal(err)
	}
	entry := only(t, path)
	if entry.State != "open" || entry.Count != 2 {
		t.Errorf("estado/conteo tras reabrir = %q/%d, want open/2", entry.State, entry.Count)
	}
}

func TestFixUnknownFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frictions.jsonl")
	result, err := Fix(path, "000000000000", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result != UnknownFP {
		t.Errorf("Fix de huella desconocida = %v, want UnknownFP", result)
	}
}

func TestFixPersistsNote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frictions.jsonl")
	start := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	record := Record{
		Class: "contract", Surface: "runner", Key: "prueba:duplicado",
		Observed: "input_needed: falta evidence", Project: "demo", Now: start,
	}
	if _, err := Add(path, record); err != nil {
		t.Fatal(err)
	}
	fp := Fingerprint(record.Surface, record.Class, record.Key)
	note := "el orquestador reenvía la operación con la evidencia"
	if result, err := Fix(path, fp, note, start.Add(time.Minute)); err != nil || result != Fixed {
		t.Fatalf("Fix = %v, %v; want Fixed", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"note":"`+note+`"`) {
		t.Errorf("fichero sin la nota en el evento fixed:\n%s", data)
	}
	entry := only(t, path)
	if entry.State != "fixed" || entry.Note != note {
		t.Errorf("estado/nota = %q/%q, want fixed/%q", entry.State, entry.Note, note)
	}

	record.Now = start.Add(2 * time.Minute)
	if _, err := Add(path, record); err != nil {
		t.Fatal(err)
	}
	if entry := only(t, path); entry.Note != "" {
		t.Errorf("nota tras reabrir = %q, want vacía", entry.Note)
	}
}

func TestFixWithoutNoteKeepsOldRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frictions.jsonl")
	// Eventos anteriores al campo note: el fixed no lo lleva y el fichero no se toca.
	content := strings.Join([]string{
		`{"t":"2026-11-28T10:00:00Z","fp":"aaaaaaaaaaaa","class":"tool","surface":"git","key":"k","observed":"o"}`,
		`{"t":"2026-11-28T10:01:00Z","fp":"aaaaaaaaaaaa","fixed":true}`,
		`{"t":"2026-11-28T10:02:00Z","fp":"bbbbbbbbbbbb","class":"tool","surface":"git","key":"k2","observed":"o2"}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	events, skipped, err := Read(path)
	if err != nil || skipped != 0 || len(events) != 3 {
		t.Fatalf("Read = %d eventos, %d omitidas, %v; want 3/0", len(events), skipped, err)
	}
	if event := events[1]; event.Note != "" || !event.Fixed {
		t.Errorf("evento fixed antiguo = %+v, want fixed sin nota", event)
	}
	if event := events[0]; event.Note != "" {
		t.Errorf("nota de un add = %q, want vacía", event.Note)
	}

	if result, err := Fix(path, "bbbbbbbbbbbb", "", time.Date(2026, 11, 28, 10, 3, 0, 0, time.UTC)); err != nil || result != Fixed {
		t.Fatalf("Fix sin nota = %v, %v; want Fixed", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"note"`) {
		t.Errorf("el cierre sin nota escribió el campo note:\n%s", data)
	}
	if entry := only(t, path); entry.State != "fixed" || entry.Note != "" {
		t.Errorf("estado/nota = %q/%q, want fixed/vacía", entry.State, entry.Note)
	}
}

func TestReadSkipsUnreadableLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frictions.jsonl")
	content := strings.Join([]string{
		`{"t":"2026-11-28T10:00:00Z","fp":"aaaaaaaaaaaa","class":"tool","surface":"git","key":"k","observed":"o","project":"p"}`,
		"esto no es json",
		`{"t":"mal","fp":"bbbbbbbbbbbb"}`,
		`{"t":"2026-11-28T10:01:00Z"}`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	events, skipped, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || skipped != 3 {
		t.Errorf("eventos/omitidas = %d/%d, want 1/3", len(events), skipped)
	}
}

func TestSelectOrdersAndFilters(t *testing.T) {
	events := []Event{
		{T: "2026-11-28T10:00:00Z", FP: "aaa", Key: "a", Observed: "a"},
		{T: "2026-11-28T10:01:00Z", FP: "bbb", Key: "b", Observed: "b"},
		{T: "2026-11-28T10:02:00Z", FP: "aaa", Key: "a", Observed: "a"},
		{T: "2026-11-28T10:03:00Z", FP: "ccc", Key: "c", Observed: "c"},
		{T: "2026-11-28T10:04:00Z", FP: "ccc", Key: "c", Observed: "c"},
		{T: "2026-11-28T10:05:00Z", FP: "bbb", Key: "b", Observed: "b"},
		{T: "2026-11-28T10:06:00Z", FP: "bbb", Key: "b", Observed: "b"},
	}
	entries := Aggregate(events)

	order := Select(entries, 1, 0)
	want := []string{"bbb", "ccc", "aaa"} // conteo 3, luego 2 con la última ocurrencia más reciente
	if len(order) != len(want) {
		t.Fatalf("orden = %v, want %v", order, want)
	}
	for i, fp := range want {
		if order[i].FP != fp {
			t.Fatalf("orden = %v, want %v", order, want)
		}
	}

	if got := Select(entries, 3, 0); len(got) != 1 || got[0].FP != "bbb" {
		t.Errorf("--min 3 = %v, want solo bbb", got)
	}
	if got := Select(entries, 1, 2); len(got) != 2 || got[0].FP != "bbb" || got[1].FP != "ccc" {
		t.Errorf("--limit 2 = %v, want bbb y ccc", got)
	}
}

func TestEntryJSONShape(t *testing.T) {
	entry := Entry{
		FP: "x", Class: "tool", Surface: "git", Key: "k", Count: 1,
		First: "t1", Last: "t2", State: "open",
		Projects: []string{"p"}, Observed: "o",
	}
	out, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatal(err)
	}
	want := []string{"fp", "class", "surface", "key", "count", "first", "last", "state", "projects", "observed"}
	if len(decoded) != len(want) {
		t.Fatalf("claves = %v, want %v", decoded, want)
	}
	for _, key := range want {
		if _, ok := decoded[key]; !ok {
			t.Errorf("falta la clave %q en %s", key, out)
		}
	}

	empty, err := json.Marshal(Select(nil, 1, 20))
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != "[]" {
		t.Errorf("lista vacía = %s, want []", empty)
	}
}

func TestOpenHidesFixed(t *testing.T) {
	entries := []Entry{{FP: "a", State: "open"}, {FP: "b", State: "fixed"}, {FP: "c", State: "open"}}
	open := Open(entries)
	if len(open) != 2 || open[0].FP != "a" || open[1].FP != "c" {
		t.Errorf("Open = %v, want solo las abiertas a y c", open)
	}
}

func TestPathHonoursEnv(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.jsonl")
	t.Setenv("ORCHESTRATOR_FRICTIONS", want)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestPathDefaultsUnderUserConfigDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("UserConfigDir no usa XDG_CONFIG_HOME en Windows")
	}
	t.Setenv("ORCHESTRATOR_FRICTIONS", "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "opencode", "frictions.jsonl"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func only(t *testing.T, path string) Entry {
	t.Helper()
	events, _, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	entries := Aggregate(events)
	if len(entries) != 1 {
		t.Fatalf("%d entradas, want 1", len(entries))
	}
	return entries[0]
}
