package store

import (
	"errors"
	"sync"
	"testing"
)

// Two independent connections mutate at once: SQLite serializes them through
// BEGIN IMMEDIATE, exactly one open execution with the fingerprint survives and
// the other is a declared duplicate, never a lost write.
func TestConcurrentPrepareSameFingerprintSerializes(t *testing.T) {
	root := t.TempDir()
	first, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	in := PrepareInput{
		Agent: "general", Goal: "mismo encargo",
		Scope: []string{"a"}, Checks: []string{"go build"}, Criteria: "compila",
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]error, 2)
	for i, s := range []*Store{first, second} {
		wg.Add(1)
		go func(i int, s *Store) {
			defer wg.Done()
			<-start
			_, results[i] = s.Prepare("ses", "", in)
		}(i, s)
	}
	close(start)
	wg.Wait()

	executed, duplicated := 0, 0
	for _, err := range results {
		var duplicate *DuplicateError
		switch {
		case err == nil:
			executed++
		case errors.As(err, &duplicate):
			duplicated++
		default:
			t.Fatalf("error inesperado: %v", err)
		}
	}
	if executed != 1 || duplicated != 1 {
		t.Fatalf("executed=%d duplicated=%d; la transacción no serializó", executed, duplicated)
	}
	if n := count(t, first, "SELECT count(*) FROM delegations"); n != 1 {
		t.Fatalf("filas tras la carrera = %d", n)
	}
}

// Distinct assignments prepared at once all commit: no write is lost.
func TestConcurrentDistinctPreparesAllCommit(t *testing.T) {
	root := t.TempDir()
	var stores []*Store
	for i := 0; i < 3; i++ {
		s, err := Open(root, true)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		stores = append(stores, s)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	errorsOut := make([]error, len(stores))
	for i, s := range stores {
		wg.Add(1)
		go func(i int, s *Store) {
			defer wg.Done()
			<-start
			_, errorsOut[i] = s.Prepare("ses", "", PrepareInput{
				Agent: "general", Goal: "encargo " + string(rune('a'+i)),
				Scope: []string{"a"}, Checks: []string{"go build"}, Criteria: "compila",
			})
		}(i, s)
	}
	close(start)
	wg.Wait()
	for i, err := range errorsOut {
		if err != nil {
			t.Fatalf("encargo %d falló: %v", i, err)
		}
	}
	if n := count(t, stores[0], "SELECT count(*) FROM delegations"); n != len(stores) {
		t.Fatalf("filas = %d; se perdió una escritura", n)
	}
}
