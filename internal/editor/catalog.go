package editor

import (
	json "encoding/json/v2"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
)

type Item struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	EN      string   `json:"en"`
	Desc    string   `json:"desc"`
	Rarity  int      `json:"rarity"`
	Rar     string   `json:"rar"`
	Tags    []string `json:"tags"`
	Derived bool     `json:"derived"`
}

type Catalog struct {
	Lingyong map[int]Item
	Relics   map[int]Item
	Version  string
}

func LoadCatalog(path string) (*Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("无法读取图鉴 data.json：%w", err)
	}
	return ParseCatalog(raw)
}

func ParseCatalog(raw []byte) (*Catalog, error) {
	if len(raw) > MaxSize {
		return nil, fmt.Errorf("图鉴数据超过 16 MB")
	}
	type entry struct {
		Item
		Source string `json:"src"`
	}
	var payload struct {
		Data struct {
			Lingyong []entry `json:"lingyong"`
			Relics   []entry `json:"relics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	cat := &Catalog{Lingyong: map[int]Item{}, Relics: map[int]Item{}, Version: Hash(raw)}
	for category, entries := range map[string][]entry{"lingyong": payload.Data.Lingyong, "relics": payload.Data.Relics} {
		target := cat.Relics
		if category == "lingyong" {
			target = cat.Lingyong
		}
		for _, e := range entries {
			if e.Source == "enum" || e.Name == "" || e.ID <= 0 || (category == "lingyong" && e.ID >= 10000) {
				continue
			}
			if _, exists := target[e.ID]; exists {
				continue
			}
			for _, tag := range e.Tags {
				e.Derived = e.Derived || tag == "衍生" || tag == "Derive"
			}
			target[e.ID] = e.Item
		}
	}
	if len(cat.Lingyong) == 0 || len(cat.Relics) == 0 {
		return nil, fmt.Errorf("data.json 的灵俑或遗物分类为空")
	}
	return cat, nil
}

func SortedItems(items map[int]Item) []Item {
	result := make([]Item, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

var slotPattern = regexp.MustCompile(`灵[俑佣]槽\s*([+＋\-−])\s*(\d+)`)

func (c *Catalog) SlotDelta(id int) int {
	match := slotPattern.FindStringSubmatch(c.Relics[id].Desc)
	if match == nil {
		return 0
	}
	n, _ := strconv.Atoi(match[2])
	if match[1] == "-" || match[1] == "−" {
		return -n
	}
	return n
}
