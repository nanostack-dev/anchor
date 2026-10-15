package anchorsdk

import (
	"context"
	"fmt"
	"iter"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// fetchPage runs one search page that starts after cursor (empty for the first
// page the builder describes) and returns its items and next_cursor.
type fetchPage[T any] func(ctx context.Context, cursor string) (items []T, nextCursor string, err error)

// allPages follows next_cursor from the page the builder starts on to the last
// page, yielding each item. It ends at the first page that carries no
// next_cursor, never by counting items, because a cursor is the only thing
// Anchor guarantees to be exact.
//
// A failed page is yielded once as the error, and the sequence ends.
func allPages[T any](ctx context.Context, op string, start string, fetch fetchPage[T]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		cursor := start
		for {
			items, next, err := fetch(ctx, cursor)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if next == "" {
				return
			}
			if next == cursor {
				var zero T
				yield(zero, fmt.Errorf("anchorsdk: %s: Anchor returned the cursor it was given", op))
				return
			}
			cursor = next
		}
	}
}

// pageAfter copies a search's pagination so a page can be fetched without
// changing the builder. A non-empty cursor replaces any offset, which Anchor
// refuses to combine with a cursor.
func pageAfter(page *nanoclient.PaginationRequest, cursor string) *nanoclient.PaginationRequest {
	next := nanoclient.PaginationRequest{}
	if page != nil {
		next = *page
	}
	if cursor != "" {
		next.Cursor = &cursor
		next.Offset = nil
	}
	return &next
}

// startCursor is the cursor a builder was given, empty when it starts at the
// first item.
func startCursor(page *nanoclient.PaginationRequest) string {
	if page == nil || page.Cursor == nil {
		return ""
	}
	return *page.Cursor
}

func derefCursor(cursor *string) string {
	if cursor == nil {
		return ""
	}
	return *cursor
}
