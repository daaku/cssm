// Package cssm provides scoped CSS for gomponents.
package cssm

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/adler32"
	"io"
	"strings"

	"github.com/maragudk/gomponents"
	h "github.com/maragudk/gomponents/html"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

func genHash(in []byte) string {
	// generate a hash for the entire ruleset. this scopes the rules with a
	// deterministic hash that changes when any of the rules change.
	hash := adler32.New()
	hash.Write(in)
	checksum := make([]byte, 4)
	binary.NativeEndian.PutUint32(checksum, hash.Sum32())
	return base64.RawURLEncoding.EncodeToString(checksum)
}

// Parses the CSS and returns the CSS processed, the key-value pair of the
// classes and scoped classes.
func Process(styles string) ([]byte, map[string]string, error) {
	// per the docs in NewInputBytes, leave room for a null byte
	in := make([]byte, len(styles), len(styles)+1)
	copy(in, styles)
	hash := genHash(in)
	zz := css.NewLexer(parse.NewInputBytes(in))
	scopedClasses := map[string]string{}

	var out bytes.Buffer
	var tmp bytes.Buffer

	addScopedName := func(rawNameBytes []byte) {
		rawName := string(rawNameBytes)
		scopedName := rawName + "_" + hash
		scopedClasses[rawName] = scopedName
		out.WriteString(scopedName)
	}

mainLoop:
	for {
		zt, data := zz.Next()
		if zt == css.ErrorToken {
			if err := zz.Err(); err == io.EOF {
				return out.Bytes(), scopedClasses, nil
			} else if err != nil {
				return nil, nil, err
			}
		}

		tmp.Write(data)

		if zt == css.ColonToken {
			zt, data := zz.Next()
			if zt == css.ErrorToken {
				continue mainLoop
			}
			if zt != css.IdentToken {
				tmp.WriteTo(&out)
				out.Write(data)
				continue mainLoop
			}
			if string(data) != "global" {
				tmp.WriteTo(&out)
				out.Write(data)
				continue mainLoop
			}
			braceCount := 0
			for {
				zt, data := zz.Next()
				if zt == css.ErrorToken {
					continue mainLoop
				}
				if zt == css.LeftBraceToken {
					if braceCount != 0 {
						out.Write(data)
					}
					braceCount++
				} else if zt == css.RightBraceToken {
					if braceCount != 1 {
						out.Write(data)
					}
					braceCount--
					if braceCount <= 0 {
						break
					}
				} else {
					out.Write(data)
				}
			}
		} else if zt == css.AtKeywordToken {
			if _, err := tmp.WriteTo(&out); err != nil {
				return nil, nil, err
			}
			zt, data := zz.Next()
			if zt == css.ErrorToken {
				continue mainLoop
			}
			out.Write(data)
			if zt != css.IdentToken {
				continue mainLoop
			}
			if string(data) != "media" {
				continue mainLoop
			}
			for {
				zt, data := zz.Next()
				if zt == css.ErrorToken {
					continue mainLoop
				}

				out.Write(data)

				braceCount := 0
				if zt == css.DelimToken && string(data) == "." {
					zt, data := zz.Next()
					if zt == css.ErrorToken {
						continue mainLoop
					}
					if zt != css.IdentToken {
						out.Write(data)
						continue
					}
					addScopedName(data)
				} else if zt == css.LeftBraceToken {
					braceCount++
				} else if zt == css.RightBraceToken {
					braceCount--
					if braceCount <= 0 {
						break
					}
				}
			}
		} else if zt == css.DelimToken && string(data) == "." {
			if _, err := tmp.WriteTo(&out); err != nil {
				return nil, nil, err
			}
			zt, data := zz.Next()
			if zt == css.ErrorToken {
				continue mainLoop
			}
			if zt != css.IdentToken {
				out.Write(data)
				continue mainLoop
			}
			addScopedName(data)
		} else {
			if _, err := tmp.WriteTo(&out); err != nil {
				return nil, nil, err
			}
		}
		tmp.Reset()
	}
}

// Collector allows for using one or more rulesets, scoping them
// deterministically, caching those results, and provides helpers to access the
// scoped classnames. This is not safe for concurrent access.
type Collector struct {
	rules  map[string]map[string]string
	styles bytes.Buffer
}

// Classes returns the classes mapped to their corresponding scoped names.
func (c *Collector) Classes(rules string) (map[string]string, error) {
	if m, found := c.rules[rules]; found {
		return m, nil
	}
	styles, mapping, err := Process(rules)
	if err != nil {
		return nil, err
	}
	c.styles.Write(styles)
	c.styles.WriteByte('\n')
	if c.rules == nil {
		c.rules = map[string]map[string]string{}
	}
	c.rules[rules] = mapping
	return mapping, nil
}

// C returns a gomponents.Node that serves as the class attribute for
// all the provided class names.
func (c *Collector) C(rules string, className ...string) gomponents.Node {
	mapping, err := c.Classes(rules)
	if err != nil {
		panic(err)
	}
	if len(className) == 1 {
		if name, found := mapping[className[0]]; found {
			return h.Class(name)
		}
		panic(fmt.Sprintf("no class found %q", className[0]))
	}
	var sb strings.Builder
	for i, name := range className {
		if i != 0 {
			sb.WriteByte(' ')
		}
		if mapped, found := mapping[name]; found {
			sb.WriteString(mapped)
		} else {
			panic(fmt.Sprintf("no class found %q", name))
		}
	}
	return h.Class(sb.String())
}

// R returns a gomponents.Node that serves as the class attribute for
// the special "root" class name.
func (c *Collector) R(rules string) gomponents.Node {
	return c.C(rules, "root")
}

// Render the collected styles.
func (c *Collector) Render(w io.Writer) error {
	if _, err := w.Write([]byte("<style>")); err != nil {
		return err
	}
	if _, err := c.styles.WriteTo(w); err != nil {
		return err
	}
	if _, err := w.Write([]byte("</style>")); err != nil {
		return err
	}
	return nil
}
