package store

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/policy"
)

// MaxSearchTerms is how many words of a query count; the rest are dropped.
const MaxSearchTerms = 8

// cjk reports whether r is written without spaces between words, so search
// splits it into character pairs. It must match the class in search_text
// (migrations/00025_search.sql).
func cjk(r rune) bool {
	return r >= 0x3040 && r <= 0x30ff ||
		r >= 0x3400 && r <= 0x4dbf ||
		r >= 0x4e00 && r <= 0x9fff ||
		r >= 0xac00 && r <= 0xd7af ||
		r >= 0xf900 && r <= 0xfaff ||
		r >= 0x20000 && r <= 0x3134f
}

// searchTerm is one word of a query, every one of which must match.
type searchTerm struct {
	// text is the word in lower case, for substring matches.
	text string
	// query and title are to_tsquery input matching the word anywhere, or in
	// the title (a blog's name) alone.
	query, title string
	// substr lets the word match blog names and hosts as a substring: always
	// for Chinese, and for other words of three characters or more, so "ai"
	// does not find every "daily".
	substr bool
}

// parseSearch splits a query into terms at spaces and at punctuation that
// stands alone. Within a term, runs of Chinese characters become adjacent
// pairs as search_text writes them, and other runs stay whole words for
// the text search parser, all in one phrase. A run of one Chinese character
// that ends the term matches as a prefix, since in a post it may be the
// first of a longer run.
func parseSearch(q string) []searchTerm {
	var terms []searchTerm
	for _, field := range strings.Fields(strings.ToLower(q)) {
		for _, word := range splitWord(field) {
			if len(terms) == MaxSearchTerms {
				return terms
			}
			terms = append(terms, termFor(word))
		}
	}
	return terms
}

// splitWord cuts a field where a run of neither Chinese characters nor
// letters and digits stands between them, such as "你好，世界".
func splitWord(field string) []string {
	var words []string
	var word strings.Builder
	for _, run := range runs(field) {
		if !cjk([]rune(run)[0]) && !strings.ContainsFunc(run, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			if word.Len() > 0 {
				words = append(words, word.String())
				word.Reset()
			}
			continue
		}
		word.WriteString(run)
	}
	if word.Len() > 0 {
		words = append(words, word.String())
	}
	return words
}

// runs splits s into maximal runs of Chinese and of other characters, as
// search_text does.
func runs(s string) []string {
	var out []string
	start := 0
	var prev bool
	for i, r := range s {
		c := cjk(r)
		if i > 0 && c != prev {
			out = append(out, s[start:i])
			start = i
		}
		prev = c
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func termFor(word string) searchTerm {
	type operand struct {
		text   string
		prefix bool
	}
	var ops []operand
	parts := runs(word)
	for i, part := range parts {
		chars := []rune(part)
		if !cjk(chars[0]) {
			ops = append(ops, operand{text: part})
			continue
		}
		for j := 0; j+1 < len(chars); j++ {
			ops = append(ops, operand{text: string(chars[j : j+2])})
		}
		last := string(chars[len(chars)-1])
		switch {
		case i+1 < len(parts):
			// The run ends here in the post too, on its last character alone.
			ops = append(ops, operand{text: last})
		case len(chars) == 1:
			ops = append(ops, operand{text: last, prefix: true})
		}
	}
	anywhere := make([]string, len(ops))
	title := make([]string, len(ops))
	for i, op := range ops {
		quoted := "'" + tsqueryEscaper.Replace(op.text) + "'"
		flags := ""
		if op.prefix {
			flags = "*"
		}
		anywhere[i] = quoted
		if flags != "" {
			anywhere[i] += ":" + flags
		}
		title[i] = quoted + ":" + flags + "A"
	}
	return searchTerm{
		text:   word,
		query:  "(" + strings.Join(anywhere, " <-> ") + ")",
		title:  "(" + strings.Join(title, " <-> ") + ")",
		substr: strings.ContainsFunc(word, cjk) || utf8.RuneCountInString(word) >= 3,
	}
}

// tsqueryEscaper quotes a word inside a quoted to_tsquery operand.
var tsqueryEscaper = strings.NewReplacer(`\`, `\\`, `'`, `''`)

// SearchQuery selects a page of search results.
type SearchQuery struct {
	Text   string
	Lang   string // a lower-case language prefix such as "zh", or empty
	Limit  int
	Cursor *SearchCursor
}

// SearchCursor is a position in search results: those that match in the
// title (a blog's name) first, then by time and id.
type SearchCursor struct {
	Top bool
	At  time.Time
	ID  int64
}

// FoundEntry is an entry a search found. Top says whether every term is in
// its title; SortAt is its publish date, or the epoch for an undated one.
type FoundEntry struct {
	StreamEntry
	Top bool
}

func joinTerms(terms []searchTerm, title bool) string {
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = t.query
		if title {
			parts[i] = t.title
		}
	}
	return strings.Join(parts, " & ")
}

// searchEntriesFrom is what entry search looks through: the entries of
// visible blogs that a blog page would show, matching every term.
const searchEntriesFrom = `
	FROM entries e
	JOIN blogs b ON b.id = e.blog_id
	WHERE e.search @@ to_tsquery('simple', @query)
	  AND ` + visible + `
	  AND ` + notSuppressed + `
	  AND (e.published_at IS NULL OR e.published_at <= now() + (@future_tolerance * interval '1 second'))
	  AND (@lang::text = '' OR lower(b.language) = @lang OR lower(b.language) LIKE @lang || '-%')`

func searchArgs(terms []searchTerm, lang string) pgx.NamedArgs {
	return pgx.NamedArgs{
		"query":            joinTerms(terms, false),
		"title_query":      joinTerms(terms, true),
		"lang":             lang,
		"unhealthy_after":  seconds(policy.UnhealthyAfter),
		"future_tolerance": seconds(policy.FutureTolerance),
	}
}

// SearchEntries returns a page of the entries matching every term of
// q.Text in their title or feed excerpt, under the blog page's visibility
// rules: those with every term in the title first, then the rest, newest
// first within each and undated ones last. See docs/design/data-model.md
// section 4.4.
func (s *Store) SearchEntries(ctx context.Context, q SearchQuery) ([]FoundEntry, error) {
	terms := parseSearch(q.Text)
	if len(terms) == 0 {
		return nil, nil
	}
	args := searchArgs(terms, q.Lang)
	args["limit"] = q.Limit
	args["has_cursor"] = q.Cursor != nil
	args["cursor_top"], args["cursor_at"], args["cursor_id"] = false, time.Time{}, int64(0)
	if q.Cursor != nil {
		args["cursor_top"], args["cursor_at"], args["cursor_id"] = q.Cursor.Top, q.Cursor.At, q.Cursor.ID
	}
	rows, err := s.pool.Query(ctx, `
		WITH hits AS (
			SELECT e.id, e.search @@ to_tsquery('simple', @title_query) AS top, coalesce(e.published_at, 'epoch') AS at
			`+searchEntriesFrom+`
		), page AS (
			SELECT id, top, at FROM hits
			WHERE NOT @has_cursor OR (top, at, id) < (@cursor_top, @cursor_at, @cursor_id)
			ORDER BY top DESC, at DESC, id DESC
			LIMIT @limit
		)
		SELECT e.id, e.blog_id, e.identity, e.url, e.title, coalesce(`+shownExcerpt+`, ''), coalesce(`+shownImage+`, ''),
		       e.published_at, e.date_trusted, e.tags, e.link_status, e.link_checked_at,
		       b.host, b.name, b.site_url, b.feed_url, b.language, p.top, p.at
		FROM page p
		JOIN entries e ON e.id = p.id
		JOIN blogs b ON b.id = e.blog_id
		ORDER BY p.top DESC, p.at DESC, p.id DESC`, args)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (FoundEntry, error) {
		var e FoundEntry
		err := row.Scan(&e.ID, &e.BlogID, &e.Identity, &e.URL, &e.Title, &e.Excerpt, &e.ImageURL,
			&e.PublishedAt, &e.DateTrusted, &e.Tags, &e.LinkStatus, &e.LinkCheckedAt,
			&e.Blog.Host, &e.Blog.Name, &e.Blog.SiteURL, &e.Blog.FeedURL, &e.Blog.Language, &e.Top, &e.SortAt)
		return e, err
	})
}

// CountSearchEntries counts what SearchEntries finds for text and lang, up
// to most; a count of most means at least that many.
func (s *Store) CountSearchEntries(ctx context.Context, text, lang string, most int) (int, error) {
	terms := parseSearch(text)
	if len(terms) == 0 {
		return 0, nil
	}
	args := searchArgs(terms, lang)
	args["most"] = most
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1 `+searchEntriesFrom+` LIMIT @most) AS hits`, args).Scan(&n)
	return n, err
}

// FoundBlog is a blog a search found. Top says whether every term is in its
// name or host.
type FoundBlog struct {
	ListedBlog
	Top bool
}

// SearchBlogs returns a page of the visible blogs matching every term of
// q.Text in their name, host or description, and how many match in all:
// those with every term in the name or host first, then the rest, by their
// last post as in the directory. A term of Chinese, or of three characters
// or more, matches the name and host as a substring; the description, and
// shorter words, as for entries.
func (s *Store) SearchBlogs(ctx context.Context, q SearchQuery) ([]FoundBlog, int, error) {
	terms := parseSearch(q.Text)
	if len(terms) == 0 {
		return nil, 0, nil
	}
	texts := make([]string, len(terms))
	queries := make([]string, len(terms))
	titles := make([]string, len(terms))
	substr := make([]bool, len(terms))
	for i, t := range terms {
		texts[i], queries[i], titles[i], substr[i] = t.text, t.query, t.title, t.substr
	}
	args := pgx.NamedArgs{
		"texts":            texts,
		"queries":          queries,
		"titles":           titles,
		"substr":           substr,
		"lang":             q.Lang,
		"limit":            q.Limit,
		"unhealthy_after":  seconds(policy.UnhealthyAfter),
		"future_tolerance": seconds(policy.FutureTolerance),
		"has_cursor":       q.Cursor != nil,
		"cursor_top":       false,
		"cursor_at":        time.Time{},
		"cursor_id":        int64(0),
	}
	if q.Cursor != nil {
		args["cursor_top"], args["cursor_at"], args["cursor_id"] = q.Cursor.Top, q.Cursor.At, q.Cursor.ID
	}
	const named = `(t.substr AND (strpos(lower(b.name), t.text) > 0 OR strpos(b.host, t.text) > 0))`
	rows, err := s.pool.Query(ctx, `
		WITH terms AS (
			SELECT * FROM unnest(@texts::text[], @queries::text[], @titles::text[], @substr::bool[]) AS t (text, query, title, substr)
		), found AS (
			SELECT b.id, b.host, b.name, b.description, b.site_url, b.feed_url, b.language, b.generator,
			       NOT EXISTS (SELECT 1 FROM terms t WHERE NOT (`+named+` OR b.search @@ to_tsquery('simple', t.title))) AS top,
			       (SELECT max(e.published_at) FROM entries e
			        WHERE e.blog_id = b.id AND e.date_trusted
			          AND `+notSuppressed+`
			          AND e.published_at <= now() + (@future_tolerance * interval '1 second')) AS last_published_at
			FROM blogs b
			WHERE `+visible+`
			  AND (@lang::text = '' OR lower(b.language) = @lang OR lower(b.language) LIKE @lang || '-%')
			  AND NOT EXISTS (SELECT 1 FROM terms t WHERE NOT (`+named+` OR b.search @@ to_tsquery('simple', t.query)))
		), keyed AS (
			SELECT *, coalesce(last_published_at, 'epoch') AS sort_at, count(*) OVER () AS total
			FROM found
		)
		SELECT id, host, name, description, site_url, feed_url, language, generator, last_published_at, sort_at, top, total
		FROM keyed
		WHERE NOT @has_cursor OR (top, sort_at, id) < (@cursor_top, @cursor_at, @cursor_id)
		ORDER BY top DESC, sort_at DESC, id DESC
		LIMIT @limit`, args)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	blogs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (FoundBlog, error) {
		var b FoundBlog
		err := row.Scan(&b.ID, &b.Host, &b.Name, &b.Description, &b.SiteURL, &b.FeedURL, &b.Language, &b.Generator,
			&b.LastPublishedAt, &b.SortAt, &b.Top, &total)
		return b, err
	})
	return blogs, total, err
}
