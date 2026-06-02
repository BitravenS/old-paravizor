package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitravens/paravizor/v1/internal/store/db"
)

func TestInsertHelpersReturnIDsWithoutDeadlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	st, err := Open(ctx, filepath.Join(t.TempDir(), "paravizor.db"), DBConfig{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	severity := "info"
	findingID, err := st.InsertFinding(ctx, &db.Finding{Scanner: "test", Severity: &severity, Title: "finding"})
	if err != nil {
		t.Fatalf("InsertFinding: %v", err)
	}
	if findingID <= 0 {
		t.Fatalf("findingID = %d, want positive", findingID)
	}

	scopeID, err := st.InsertScopeRule(ctx, "example.com", "exact", true)
	if err != nil {
		t.Fatalf("InsertScopeRule: %v", err)
	}
	if scopeID <= 0 {
		t.Fatalf("scopeID = %d, want positive", scopeID)
	}

	batchID, err := st.InsertBatch(ctx, "node", 1)
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	if batchID <= 0 {
		t.Fatalf("batchID = %d, want positive", batchID)
	}

	processID, err := st.InsertProcess(ctx, &db.Process{Name: "tool", Command: "tool", NodeID: "node"})
	if err != nil {
		t.Fatalf("InsertProcess: %v", err)
	}
	if processID <= 0 {
		t.Fatalf("processID = %d, want positive", processID)
	}

	urlID, err := st.InsertURL(ctx, "https://example.com/a.js", "test", nil, nil)
	if err != nil {
		t.Fatalf("InsertURL: %v", err)
	}
	fileID, err := st.InsertDownloadedFile(ctx, urlID, "/tmp/a.js", "js", nil, nil)
	if err != nil {
		t.Fatalf("InsertDownloadedFile: %v", err)
	}
	if fileID <= 0 {
		t.Fatalf("fileID = %d, want positive", fileID)
	}

	noteID, err := st.InsertNote(ctx, "note")
	if err != nil {
		t.Fatalf("InsertNote: %v", err)
	}
	if noteID <= 0 {
		t.Fatalf("noteID = %d, want positive", noteID)
	}
}
