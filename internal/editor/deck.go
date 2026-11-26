package editor

import (
	"bytes"
	"encoding/json/jsontext"
	"fmt"
	"strconv"
	"strings"
)

const deckField = "WorkbenchPaiMianBindList"

type PaiMian struct {
	HuaSe  int `json:"huaSe"`
	Number int `json:"number"`
	Feng   int `json:"feng"`
	Jian   int `json:"jian"`
}

type TileDefinition struct {
	Key     string  `json:"key"`
	Name    string  `json:"name"`
	Group   string  `json:"group"`
	Asset   string  `json:"asset"`
	PaiMian PaiMian `json:"paiMian"`
}

type DeckValue struct {
	Key    string `json:"key"`
	Score  string `json:"score"`
	BaoID  int    `json:"baoId"`
	LingID int    `json:"lingId"`
}

type DeckCard struct {
	DeckValue
	Name     string `json:"name"`
	Group    string `json:"group"`
	Asset    string `json:"asset"`
	Existing bool   `json:"existing"`
}

type deckRow struct {
	raw   jsontext.Value
	doc   *document
	value DeckValue
}

type deckState struct {
	rows  int
	cards []DeckCard
	byKey map[string]deckRow
}

var canonicalTiles = makeCanonicalTiles()

func makeCanonicalTiles() []TileDefinition {
	tiles := make([]TileDefinition, 0, 34)
	for _, suit := range []struct {
		key, name, group, asset string
		huase                   int
	}{
		{"wan", "万", "万", "Wan", 1},
		{"tong", "筒", "筒", "Bing", 2},
		{"suo", "索", "索", "Tiao", 3},
	} {
		for number := 1; number <= 9; number++ {
			tiles = append(tiles, TileDefinition{
				Key: suit.key + strconv.Itoa(number), Name: strconv.Itoa(number) + suit.name,
				Group: suit.group, Asset: suit.asset + strconv.Itoa(number) + ".png",
				PaiMian: PaiMian{HuaSe: suit.huase, Number: number},
			})
		}
	}
	for _, honor := range []TileDefinition{
		{Key: "dong", Name: "东", Group: "字", Asset: "Dong.png", PaiMian: PaiMian{Feng: 1}},
		{Key: "nan", Name: "南", Group: "字", Asset: "Nan.png", PaiMian: PaiMian{Feng: 2}},
		{Key: "xi", Name: "西", Group: "字", Asset: "XI.png", PaiMian: PaiMian{Feng: 3}},
		{Key: "bei", Name: "北", Group: "字", Asset: "Bei.png", PaiMian: PaiMian{Feng: 4}},
		{Key: "zhong", Name: "中", Group: "字", Asset: "Zhong.png", PaiMian: PaiMian{Jian: 1}},
		{Key: "fa", Name: "发", Group: "字", Asset: "Fa.png", PaiMian: PaiMian{Jian: 3}},
		{Key: "bai", Name: "白", Group: "字", Asset: "Bai.png", PaiMian: PaiMian{Jian: 2}},
	} {
		tiles = append(tiles, honor)
	}
	return tiles
}

func CanonicalTiles() []TileDefinition {
	return append([]TileDefinition(nil), canonicalTiles...)
}

func tileByKey(key string) (TileDefinition, bool) {
	for _, tile := range canonicalTiles {
		if tile.Key == key {
			return tile, true
		}
	}
	return TileDefinition{}, false
}

func tileKey(p PaiMian) (string, bool) {
	for _, tile := range canonicalTiles {
		if tile.PaiMian == p {
			return tile.Key, true
		}
	}
	return "", false
}

func parseDeck(raw jsontext.Value) (*deckState, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		return nil, fmt.Errorf("%s 必须是数组", deckField)
	}
	rows, err := decode[[]jsontext.Value](raw)
	if err != nil {
		return nil, fmt.Errorf("%s 格式不正确：%w", deckField, err)
	}
	state := &deckState{rows: len(rows), byKey: make(map[string]deckRow, len(rows))}
	for index, rowRaw := range rows {
		doc, err := parse(rowRaw)
		if err != nil {
			return nil, fmt.Errorf("%s 第 %d 条格式不正确：%w", deckField, index+1, err)
		}
		for _, key := range []string{"PaiMian", "DeltaBaseScore", "BaoId", "LingId"} {
			if _, exists := doc.values[key]; !exists {
				return nil, fmt.Errorf("%s 第 %d 条缺少 %s", deckField, index+1, key)
			}
		}
		pai, err := decode[PaiMian](doc.values["PaiMian"])
		if err != nil {
			return nil, fmt.Errorf("%s 第 %d 条牌面格式不正确", deckField, index+1)
		}
		key, known := tileKey(pai)
		if !known {
			return nil, fmt.Errorf("%s 第 %d 条包含未知牌面 %+v", deckField, index+1, pai)
		}
		if _, duplicate := state.byKey[key]; duplicate {
			return nil, fmt.Errorf("%s 中的 %s 重复", deckField, key)
		}
		score := strings.TrimSpace(string(doc.values["DeltaBaseScore"]))
		if _, err = deckScore(score); err != nil {
			return nil, fmt.Errorf("%s 第 %d 条牌面分无效：%w", deckField, index+1, err)
		}
		baoID, err := decode[int](doc.values["BaoId"])
		if err != nil || baoID < 0 {
			return nil, fmt.Errorf("%s 第 %d 条 BaoId 无效", deckField, index+1)
		}
		lingID, err := decode[int](doc.values["LingId"])
		if err != nil || lingID < 0 {
			return nil, fmt.Errorf("%s 第 %d 条 LingId 无效", deckField, index+1)
		}
		state.byKey[key] = deckRow{raw: rowRaw, doc: doc, value: DeckValue{Key: key, Score: score, BaoID: baoID, LingID: lingID}}
	}
	state.cards = make([]DeckCard, 0, len(canonicalTiles))
	for _, tile := range canonicalTiles {
		value := DeckValue{Key: tile.Key, Score: "0.0"}
		_, existing := state.byKey[tile.Key]
		if existing {
			value = state.byKey[tile.Key].value
		}
		state.cards = append(state.cards, DeckCard{DeckValue: value, Name: tile.Name, Group: tile.Group, Asset: tile.Asset, Existing: existing})
	}
	return state, nil
}

func normalizeDeckRequest(values []DeckValue, before *deckState, catalog *Catalog) (map[string]DeckValue, error) {
	if len(values) != len(canonicalTiles) {
		return nil, fmt.Errorf("牌库保存时必须包含 34 种牌面，当前为 %d 种", len(values))
	}
	result := make(map[string]DeckValue, len(values))
	for _, value := range values {
		if _, known := tileByKey(value.Key); !known {
			return nil, fmt.Errorf("牌库包含未知牌面 %q", value.Key)
		}
		if _, duplicate := result[value.Key]; duplicate {
			return nil, fmt.Errorf("牌库中的 %s 重复", value.Key)
		}
		score, err := deckScore(value.Score)
		if err != nil {
			return nil, fmt.Errorf("%s 的牌面分无效：%w", value.Key, err)
		}
		old := DeckValue{Key: value.Key, Score: "0.0"}
		if row, exists := before.byKey[value.Key]; exists {
			old = row.value
		}
		if value.BaoID < 0 || (value.BaoID != 0 && value.BaoID != old.BaoID && catalog.Baopai[value.BaoID].ID == 0) {
			return nil, fmt.Errorf("%s 的宝牌 ID %d 不存在于当前图鉴", value.Key, value.BaoID)
		}
		if value.LingID < 0 || (value.LingID != 0 && value.LingID != old.LingID && catalog.Pailing[value.LingID].ID == 0) {
			return nil, fmt.Errorf("%s 的牌灵 ID %d 不存在于当前图鉴", value.Key, value.LingID)
		}
		value.Score = score
		result[value.Key] = value
	}
	return result, nil
}

func buildDeck(before *deckState, values map[string]DeckValue) (string, []string, error) {
	rows := make([]string, 0, len(canonicalTiles))
	changed := []string{}
	for _, tile := range canonicalTiles {
		value := values[tile.Key]
		old := DeckValue{Key: tile.Key, Score: "0.0"}
		if row, exists := before.byKey[tile.Key]; exists {
			old = row.value
			replacements := map[string]string{}
			if !equalNumber(old.Score, value.Score) {
				replacements["DeltaBaseScore"] = value.Score
			}
			if old.BaoID != value.BaoID {
				replacements["BaoId"] = strconv.Itoa(value.BaoID)
			}
			if old.LingID != value.LingID {
				replacements["LingId"] = strconv.Itoa(value.LingID)
			}
			rows = append(rows, string(row.doc.replace(replacements)))
		} else {
			rows = append(rows, fmt.Sprintf(`{"PaiMian":%s,"DeltaBaseScore":%s,"BaoId":%d,"LingId":%d}`,
				marshal(tile.PaiMian), value.Score, value.BaoID, value.LingID))
		}
		if !equalNumber(old.Score, value.Score) || old.BaoID != value.BaoID || old.LingID != value.LingID {
			changed = append(changed, tile.Name)
		}
	}
	return "[" + strings.Join(rows, ",") + "]", changed, nil
}
