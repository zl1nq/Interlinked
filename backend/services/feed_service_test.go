package services

import (
	"testing"
	"time"

	"feed/models"
)

func TestParseTimelineCursorRoundTrip(t *testing.T) {
	createdAt := time.UnixMilli(1234)
	cursor := buildTimelineCursor(createdAt, 42)

	parsedTime, parsedID := parseTimelineCursor(cursor)
	if !parsedTime.Equal(createdAt) {
		t.Fatalf("expected time %v, got %v", createdAt, parsedTime)
	}
	if parsedID != 42 {
		t.Fatalf("expected id 42, got %d", parsedID)
	}
}

func TestParseTimelineCursorInvalidReturnsDefault(t *testing.T) {
	parsedTime, parsedID := parseTimelineCursor("bad-cursor")
	if parsedID == 0 {
		t.Fatalf("expected default cursor id, got 0")
	}
	if parsedTime.IsZero() {
		t.Fatalf("expected non-zero default time")
	}
}

func TestFilterTimelineCandidatesRespectsCursorBoundary(t *testing.T) {
	svc := &FeedService{}
	cursorTime := time.Unix(100, 0)
	cursorID := uint(10)
	feeds := []models.Feed{
		{ID: 9, CreatedAt: cursorTime},
		{ID: 10, CreatedAt: cursorTime},
		{ID: 11, CreatedAt: cursorTime},
		{ID: 12, CreatedAt: time.Unix(99, 0)},
		{ID: 13, CreatedAt: time.Unix(101, 0)},
	}

	ids := svc.filterTimelineCandidates(feeds, cursorTime, cursorID)
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d: %#v", len(ids), ids)
	}
	if ids[0] != 9 || ids[1] != 12 {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}

func TestMergeAndDedup(t *testing.T) {
	svc := &FeedService{}
	result := svc.mergeAndDedup([]uint{1, 2, 2}, []uint{2, 3}, []uint{1, 4})
	if len(result) != 4 {
		t.Fatalf("expected 4 ids, got %d: %#v", len(result), result)
	}
}

func TestPickTimelinePage(t *testing.T) {
	feeds := []models.Feed{{ID: 1}, {ID: 2}, {ID: 3}}
	page, hasMore := pickTimelinePage(feeds, 2)
	if len(page) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page))
	}
	if !hasMore {
		t.Fatalf("expected hasMore=true")
	}

	page, hasMore = pickTimelinePage(feeds[:1], 2)
	if len(page) != 1 || hasMore {
		t.Fatalf("unexpected single-page result: len=%d hasMore=%v", len(page), hasMore)
	}
}
