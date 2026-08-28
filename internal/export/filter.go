// Package export post-processes JSON API responses for display: jq filtering
// and Go templates, mirroring what `gh` offers.
package export

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/itchyny/gojq"
)

// FilterJSON runs a jq expression over the JSON read from r and writes the
// results to w. Strings are printed bare, everything else as compact JSON —
// the same convention jq itself uses with -r.
func FilterJSON(w io.Writer, r io.Reader, query string) error {
	q, err := gojq.Parse(query)
	if err != nil {
		return fmt.Errorf("invalid jq expression: %w", err)
	}

	var input any
	dec := json.NewDecoder(r)
	dec.UseNumber()
	if err := dec.Decode(&input); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	code, err := gojq.Compile(q)
	if err != nil {
		return fmt.Errorf("invalid jq expression: %w", err)
	}

	iter := code.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			var haltErr *gojq.HaltError
			if ok := asHalt(err, &haltErr); ok && haltErr.Value() == nil {
				break
			}
			return err
		}
		if err := writeValue(w, v); err != nil {
			return err
		}
	}
	return nil
}

func asHalt(err error, target **gojq.HaltError) bool {
	he, ok := err.(*gojq.HaltError)
	if ok {
		*target = he
	}
	return ok
}

func writeValue(w io.Writer, v any) error {
	switch tv := v.(type) {
	case nil:
		_, err := fmt.Fprintln(w, "null")
		return err
	case string:
		_, err := fmt.Fprintln(w, tv)
		return err
	default:
		data, err := gojq.Marshal(v)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, strings.TrimRight(string(data), "\n"))
		return err
	}
}
