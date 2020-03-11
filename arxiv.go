// Package arxiv provides an abstraction over the arXiv API.
//
// Please note that you are not allowed to make more than 1 request every 3 seconds.
// For more information see the terms of use: https://arxiv.org/help/api/tou.
package arxiv

import (
	"strings"
	"time"
)

// Type Author represents information about an author.
//
// The author's name is always First Middle Last. First and Middle may be abbreviated to F. M.
type Author struct {
	Name        string
	Affiliation string
}

// Type Paper represents metadata information about a paper.
type Paper struct {
	URL string
	DOI string

	Updated   time.Time
	Published time.Time

	Title      string
	Summary    string
	Categories []string
	Journal    string

	Authors []Author

	// Author's comment
	Comment string
	// Number of pages. 0 if not specified.
	Pages uint
}

// Method ID returns the arXiv ID of the paper.
// For more information, see https://arxiv.org/help/arxiv_identifier.
func (p *Paper) ID() string {
	i := strings.Index(p.URL, "/abs/")
	if i < 0 {
		return ""
	}
	i += len("/abs/")

	id := p.URL[i:]

	if strings.Index(id, "/") == -1 {
		id = "arXiv:" + id
	}

	return id
}

// Type Query represents the parameters that are sent to a search query.
type Query struct {
	// search_query parameter. For more information
	// on its construction, see
	// https://arxiv.org/help/api/user-manual#query_details
	// Optional
	Query string

	// List of arXiv IDs
	// Optional
	IDList []string

	// Index of first search result
	// Required
	Start uint

	// Maximum number of results. Must satisfy 0 < Max <= 30000
	// Required
	Max uint
}

// Function NewQuery is a convenience method that constructs a search query from a search term.
func NewQuery(searchTerm string, start uint, max uint) *Query {
	return &Query{
		Query:  "all:" + searchTerm,
		IDList: nil,
		Start:  start,
		Max:    max,
	}
}
