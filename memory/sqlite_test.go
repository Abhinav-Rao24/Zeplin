package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestSQLiteStore(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_zeplin.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	studentID := "test_student"

	// 1. Test Conversation Turns (MemoryStore interface)
	msgUser := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "How do I play G major?"}
	msgAssistant := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Place your index on the 2nd fret of A string."}

	if err := store.Append(ctx, studentID, msgUser, msgAssistant); err != nil {
		t.Fatalf("Failed to append messages: %v", err)
	}

	turns, err := store.Read(ctx, studentID)
	if err != nil {
		t.Fatalf("Failed to read messages: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("Expected 2 turns, got %d", len(turns))
	}
	if turns[0].Content != msgUser.Content || turns[1].Content != msgAssistant.Content {
		t.Errorf("Turn contents do not match expected")
	}

	// 2. Test Curriculum Progress Persistence
	if err := store.SaveCurriculumProgress(studentID, 3, 2, []int{1, 2}); err != nil {
		t.Fatalf("Failed to save progress: %v", err)
	}

	stepID, streak, completed, err := store.GetCurriculumProgress(studentID)
	if err != nil {
		t.Fatalf("Failed to get progress: %v", err)
	}
	if stepID != 3 || streak != 2 || len(completed) != 2 {
		t.Errorf("Progress mismatch: step=%d streak=%d completed=%v", stepID, streak, completed)
	}

	// 3. Test Mistake Profiling (upsert increment)
	if err := store.RecordMistake(studentID, "C_Major", "muted_string", "B_string_muted"); err != nil {
		t.Fatalf("Failed to record mistake: %v", err)
	}
	// Record again to verify count increments to 2
	if err := store.RecordMistake(studentID, "C_Major", "muted_string", "B_string_muted"); err != nil {
		t.Fatalf("Failed to record second mistake: %v", err)
	}

	mistakes, err := store.GetTopMistakes(studentID, 5)
	if err != nil {
		t.Fatalf("Failed to get top mistakes: %v", err)
	}
	if len(mistakes) != 1 {
		t.Fatalf("Expected 1 aggregated mistake, got %d", len(mistakes))
	}
	if mistakes[0].OccurrenceCount != 2 {
		t.Errorf("Expected occurrence count 2, got %d", mistakes[0].OccurrenceCount)
	}

	// 4. Test Session Logging
	now := time.Now()
	rec := SessionRecord{
		ID:                "sess-123",
		StudentID:         studentID,
		StartedAt:         now.Add(-15 * time.Minute),
		EndedAt:           now,
		DurationSec:       900,
		TotalStrums:       40,
		CleanStrums:       32,
		TuningOffsetCents: -5.2,
	}
	if err := store.SaveSession(rec); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	summary, err := store.GetStudentSummary(studentID)
	if err != nil {
		t.Fatalf("Failed to get student summary: %v", err)
	}
	if summary.TotalSessions != 1 || summary.TotalStrums != 40 || summary.CleanStrums != 32 {
		t.Errorf("Summary mismatch: %+v", summary)
	}
	expectedAccuracy := 32.0 / 40.0
	if summary.OverallAccuracy != expectedAccuracy {
		t.Errorf("Expected accuracy %f, got %f", expectedAccuracy, summary.OverallAccuracy)
	}
}

func init() {
	_ = os.Setenv("TEST", "true")
}
