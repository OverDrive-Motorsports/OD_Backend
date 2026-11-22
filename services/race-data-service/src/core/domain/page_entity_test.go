/**
##
## OverDrive 2026
## All Technical rights reserved
##
## page_entity_test.go - Tests for the pagination helpers.
##
*/

package domain

import "testing"

// TestTrimPage_LastPage proves a repository result within the limit is returned whole with no
// next offset.
func TestTrimPage_LastPage(t *testing.T) {
	page := Page{Offset: 20, Limit: 3}
	rows, next := TrimPage([]int{1, 2, 3}, page)
	if len(rows) != 3 || next != nil {
		t.Fatalf("got %v next=%v, want 3 rows and no next page", rows, next)
	}
}

// TestTrimPage_MoreRows proves the extra row read by Fetch is cut and the next offset is
// offset+limit.
func TestTrimPage_MoreRows(t *testing.T) {
	page := Page{Offset: 20, Limit: 3}
	if page.Fetch() != 4 {
		t.Fatalf("Fetch() = %d, want limit+1", page.Fetch())
	}
	rows, next := TrimPage([]int{1, 2, 3, 4}, page)
	if len(rows) != 3 || next == nil || *next != 23 {
		t.Fatalf("got %v next=%v, want 3 rows and next offset 23", rows, next)
	}
}

// TestDefaultPage proves the default is the first page at the maximum size.
func TestDefaultPage(t *testing.T) {
	if page := DefaultPage(); page.Offset != 0 || page.Limit != MaxPageLimit {
		t.Fatalf("DefaultPage() = %+v", page)
	}
}
