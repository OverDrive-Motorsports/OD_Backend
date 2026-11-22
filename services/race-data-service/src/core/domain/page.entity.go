/**
##
## OverDrive 2026
## All Technical rights reserved
##
## page.entity.go - Pagination of the large sample lists.
##
*/

package domain

// MaxPageLimit is the most rows a sample list returns in one response, and the default when the
// client sends no limit. A full race of one driver (car data or location) fits in one page; a
// session-wide list of every driver does not and must be paged.
const MaxPageLimit = 50000

// Page selects a slice of an ordered sample list.
type Page struct {
	Offset int
	Limit  int
}

// DefaultPage is the first page at the maximum size.
func DefaultPage() Page {
	return Page{Limit: MaxPageLimit}
}

// Fetch is how many rows a repository reads for the page: one more than the limit, so the
// caller knows whether a next page exists without counting.
func (p Page) Fetch() int {
	return p.Limit + 1
}

// TrimPage cuts the extra row read by Fetch. It returns the page rows and, when more rows
// exist, the offset of the next page.
func TrimPage[T any](rows []T, page Page) ([]T, *int) {
	if len(rows) <= page.Limit {
		return rows, nil
	}
	next := page.Offset + page.Limit
	return rows[:page.Limit], &next
}
