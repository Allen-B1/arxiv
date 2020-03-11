package arxiv

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	spaceAtom  = "http://www.w3.org/2005/Atom"
	spaceArXiv = "http://arxiv.org/schemas/atom"
)

// Type SearchError represents an error resulting from a malformed request.
type SearchError string

func (e SearchError) Error() string {
	return string(e)
}

func Search(q *Query) ([]Paper, error) {
	values := url.Values{}
	if q.Query != "" {
		values.Set("search_query", q.Query)
	}
	if q.IDList != nil {
		str := ""
		for _, id := range q.IDList {
			str += id
			str += ","
		}
		str = str[:len(str)-1]
		values.Set("id_list", str)
	}
	values.Set("start", fmt.Sprint(q.Start))
	values.Set("max_results", fmt.Sprint(q.Max))

	//	log.Println("http://export.arxiv.org/api/query?" + values.Encode())

	resp, err := http.Get("http://export.arxiv.org/api/query?" + values.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}

	d := xml.NewDecoder(resp.Body)
	out := make([]Paper, 0)
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse search results: %w", err)
		}

		elem, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		if elem.Name.Local == "entry" && elem.Name.Space == spaceAtom {
			out = append(out, Paper{})
			paper := &out[len(out)-1]
			err := parsePaper(d, paper)
			if err != nil {
				return nil, err
			}

			if strings.EqualFold(paper.Title, "error") {
				return nil, SearchError(paper.Summary)
			}
		}
	}

	return out, nil
}

func getInnerValue(d *xml.Decoder, ret *string) error {
	token, err := d.Token()
	if err != nil {
		return fmt.Errorf("failed to parse search results: %w", err)
	}

	// if next token is CharData
	chars, ok := token.(xml.CharData)
	if ok {
		*ret = string(chars)

		// set token to EndElement
		token, err = d.Token()
		if err != nil {
			return fmt.Errorf("failed to parse search results: %w", err)
		}
	}

	// parse EndElement
	_, ok = token.(xml.EndElement)
	if !ok {
		return fmt.Errorf("failed to parse search results: expected EndElement")
	}
	return nil
}

func parseAuthor(d *xml.Decoder, author *Author) error {
	for {
		token, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to parse search results: %w", err)
		}

		var elem xml.StartElement
		switch tk := token.(type) {
		case xml.StartElement:
			elem = tk
		case xml.EndElement:
			if tk.Name.Local == "author" {
				return nil
			}
			continue
		default:
			continue
		}

		switch elem.Name.Local {
		case "name":
			err = getInnerValue(d, &author.Name)
			if err != nil {
				return err
			}
		case "affiliation":
			err = getInnerValue(d, &author.Affiliation)
			if err != nil {
				return err
			}
		}
	}
}

var spaceRe = regexp.MustCompile("[\\s\n]+")

func parsePaper(d *xml.Decoder, paper *Paper) error {
	for {
		token, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to parse search results: %w", err)
		}

		var elem xml.StartElement
		switch tk := token.(type) {
		case xml.StartElement:
			elem = tk
		case xml.EndElement:
			if tk.Name.Local == "entry" {
				return nil
			}
			continue
		default:
			continue
		}

		//		log.Println("space=" + elem.Name.Space + ", name=" + elem.Name.Local)

		switch elem.Name.Space {
		case spaceAtom:
			switch elem.Name.Local {
			case "id":
				err = getInnerValue(d, &paper.URL)
				if err != nil {
					return err
				}
			case "title":
				err = getInnerValue(d, &paper.Title)
				if err != nil {
					return err
				}
				paper.Title = string(spaceRe.ReplaceAll([]byte(paper.Title), []byte(" ")))
				paper.Title = strings.Trim(paper.Title, " \t\n")
			case "summary":
				err = getInnerValue(d, &paper.Summary)
				if err != nil {
					return err
				}
				paper.Summary = string(spaceRe.ReplaceAll([]byte(paper.Summary), []byte(" ")))
				paper.Summary = strings.Trim(paper.Summary, " \n\t")
			case "updated":
				str := ""
				err = getInnerValue(d, &str)
				if err != nil {
					return err
				}
				time_, err := time.Parse(time.RFC3339, str)
				if err == nil {
					paper.Updated = time_
				}
			case "published":
				str := ""
				err = getInnerValue(d, &str)
				if err != nil {
					return err
				}
				time_, err := time.Parse(time.RFC3339, str)
				if err == nil {
					paper.Published = time_
				}
			case "author":
				paper.Authors = append(paper.Authors, Author{})
				err = parseAuthor(d, &paper.Authors[len(paper.Authors)-1])
				if err != nil {
					return err
				}
			case "category":
				for _, attr := range elem.Attr {
					if attr.Name.Local == "term" {
						paper.Categories = append(paper.Categories, attr.Value)
						break
					}
				}

				end, err := d.Token()
				if err != nil {
					return fmt.Errorf("failed to parse search results: %w", err)
				}
				if _, ok := end.(xml.EndElement); !ok {
					return fmt.Errorf("failed to parse search results: expected EndElement")
				}
			}
		case spaceArXiv:
			switch elem.Name.Local {
			case "doi":
				err = getInnerValue(d, &paper.DOI)
				if err != nil {
					return err
				}

			case "journal_ref":
				err = getInnerValue(d, &paper.Journal)
				if err != nil {
					return err
				}

			case "comment":
				err = getInnerValue(d, &paper.Comment)
				if err != nil {
					return err
				}

				fields := strings.Fields(paper.Comment)
				for i, field := range fields {
					fieldParsed := strings.ToLower(strings.Trim(field, ","))
					if (fieldParsed == "pages" || fieldParsed == "page") && i > 0 {
						pages, err := strconv.ParseUint(fields[i-1], 10, 64)
						if err == nil {
							paper.Pages = uint(pages)
						}
					}
				}
			}
		}
	}
	return nil
}
