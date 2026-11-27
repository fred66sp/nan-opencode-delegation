package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCorruptDatabaseFailsMutationsAndDeclaresCorrupt(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".run", "orchestrator.db")
	if err := os.WriteFile(path, []byte("esto no es una base de datos"), 0o644); err != nil {
		t.Fatal(err)
	}

	ro, err := Open(root, false)
	if err != nil {
		t.Fatal(err)
	}
	state, err := ro.SessionPhase("ses")
	if err != nil {
		t.Fatalf("la consulta no debe fallar: %v", err)
	}
	if !state.Corrupt || state.Valid || !state.Exists {
		t.Fatalf("estado corrupto = %+v", state)
	}
	ro.Close()

	rw, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	if !rw.Corrupt() {
		t.Fatal("la base dañada debía marcarse corrupta")
	}
	if _, err := rw.Next("ses", "no debe aplicarse", ""); !errors.Is(err, ErrStateCorrupt) {
		t.Fatalf("mutación sobre base dañada = %v", err)
	}
}

func TestRecoverArchivesRescuesAndDeclares(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".run")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "orchestrator.db")
	if err := os.WriteFile(path, []byte("basura sin cabecera SQLite"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	report, err := s.Recover("ses")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	s.Close()
	if !report.Corrupt || report.Archived == "" {
		t.Fatalf("informe = %+v", report)
	}
	if _, err := os.Stat(report.Archived); err != nil {
		t.Fatalf("no se archivó el fichero dañado: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no se reconstruyó la base: %v", err)
	}
	if len(report.Lost) == 0 {
		t.Fatalf("una base ilegible debe declarar tablas perdidas: %+v", report)
	}
	if report.Phase != 1 {
		t.Fatalf("fase reconstruida = %d", report.Phase)
	}

	fresh, err := Open(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	state, err := fresh.SessionPhase("ses")
	if err != nil {
		t.Fatal(err)
	}
	if !state.Exists || state.Phase != 1 || !state.Valid {
		t.Fatalf("estado tras recover = %+v", state)
	}
}

func TestRecoverOnHealthyDatabaseChangesNothing(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Next("ses", "evidencia", ""); err != nil {
		t.Fatal(err)
	}
	report, err := s.Recover("ses")
	if err != nil {
		t.Fatal(err)
	}
	if report.Corrupt || report.Archived != "" {
		t.Fatalf("una base sana no se archiva: %+v", report)
	}
	state, _ := s.SessionPhase("ses")
	if state.Phase != 2 {
		t.Fatalf("el recover sano tocó el estado: %+v", state)
	}
}
