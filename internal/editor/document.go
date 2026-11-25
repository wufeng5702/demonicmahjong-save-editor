package editor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

const MaxSize = 16 << 20

type span struct{ start, end int }

// document records exact root-value boundaries. Untouched values are never marshalled.
type document struct {
	text   string
	bom    bool
	values map[string]json.RawMessage
	spans  map[string]span
	order  []string
}

func parse(raw []byte) (*document, error) {
	if len(raw) > MaxSize || !utf8.Valid(raw) {
		return nil, fmt.Errorf("文件超过 16 MB，或不是 UTF-8 JSON")
	}
	d := &document{bom: bytes.HasPrefix(raw, []byte{239, 187, 191}), values: make(map[string]json.RawMessage), spans: make(map[string]span)}
	if d.bom {
		raw = raw[3:]
	}
	d.text = string(raw)
	dec := json.NewDecoder(strings.NewReader(d.text))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("存档必须是有效的 JSON 对象")
	}
	for dec.More() {
		token, err = dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("JSON 键不是字符串")
		}
		if _, exists := d.values[key]; exists {
			return nil, fmt.Errorf("JSON 含重复键 %s，拒绝编辑", key)
		}
		start := int(dec.InputOffset())
		for start < len(d.text) && strings.ContainsRune(" \r\n\t:", rune(d.text[start])) {
			start++
		}
		var value json.RawMessage
		if err = dec.Decode(&value); err != nil {
			return nil, err
		}
		d.values[key] = value
		d.spans[key] = span{start, int(dec.InputOffset())}
		d.order = append(d.order, key)
	}
	if _, err = dec.Token(); err != nil {
		return nil, err
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("JSON 对象后还有多余内容")
	}
	return d, nil
}

func (d *document) replace(changes map[string]string) []byte {
	keys := make([]string, 0, len(changes))
	for key := range changes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return d.spans[keys[i]].start > d.spans[keys[j]].start })
	text := d.text
	for _, key := range keys {
		p := d.spans[key]
		text = text[:p.start] + changes[key] + text[p.end:]
	}
	if d.bom {
		text = "\xef\xbb\xbf" + text
	}
	return []byte(text)
}

func Hash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func marshal(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err) // Only fixed, JSON-safe internal structs and integer lists use this helper.
	}
	return string(raw)
}
