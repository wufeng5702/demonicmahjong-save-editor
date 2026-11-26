package editor

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type RelicChoice struct {
	Ref *int `json:"ref,omitempty"`
	ID  *int `json:"id,omitempty"`
}

type HeldRelic struct {
	Ref int `json:"ref"`
	ID  int `json:"id"`
}

type Snapshot struct {
	Path      string            `json:"path"`
	Hash      string            `json:"hash"`
	Version   string            `json:"version"`
	Fields    map[string]string `json:"fields"`
	Equipped  []int             `json:"equipped"`
	Warehouse []int             `json:"warehouse"`
	Relics    []HeldRelic       `json:"relics"`
	Deck      []DeckCard        `json:"deck"`
	DeckRows  int               `json:"deckRows"`
}

type Request struct {
	Path        string            `json:"path"`
	Hash        string            `json:"hash"`
	Fields      map[string]string `json:"fields"`
	Equipped    []int             `json:"equipped"`
	Warehouse   []int             `json:"warehouse"`
	Relics      []RelicChoice     `json:"relics"`
	Deck        *[]DeckValue      `json:"deck,omitempty"`
	PreviewHash string            `json:"previewHash"`
}

type Change struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type Preview struct {
	Changes     []Change `json:"changes"`
	Warnings    []string `json:"warnings"`
	SlotDelta   int      `json:"slotDelta"`
	Slots       int      `json:"slots"`
	PreviewHash string   `json:"previewHash"`
}

func decode[T any](raw jsontext.Value) (T, error) {
	var value T
	err := json.Unmarshal(raw, &value)
	return value, err
}

func validate(d *document) error {
	for _, field := range Fields {
		raw, exists := d.values[field.Key]
		if !exists || len(raw) == 0 || raw[0] == '"' {
			return fmt.Errorf("缺少有效数值字段 %s", field.Key)
		}
		if field.Integer {
			if _, err := decode[int](raw); err != nil {
				return fmt.Errorf("%s 不是整数", field.Key)
			}
		} else if raw.Kind() != jsontext.KindNumber {
			return fmt.Errorf("%s 不是数值", field.Key)
		}
	}
	for _, key := range []string{"AllXcp", "AllXcpRepository"} {
		if !bytes.HasPrefix(d.values[key], []byte("[")) {
			return fmt.Errorf("缺少列表 %s", key)
		}
		if _, err := decode[[]int](d.values[key]); err != nil {
			return fmt.Errorf("%s 不是灵俑 ID 列表", key)
		}
	}
	if _, err := parse(d.values["Xcp"]); err != nil {
		return fmt.Errorf("Xcp 成长记录格式不受支持：%w", err)
	}
	if !bytes.HasPrefix(d.values["RelicList"], []byte("[")) {
		return fmt.Errorf("缺少遗物列表 RelicList")
	}
	relics, err := decode[[]jsontext.Value](d.values["RelicList"])
	if err != nil {
		return err
	}
	for _, raw := range relics {
		if _, err = relicID(raw); err != nil {
			return err
		}
	}
	deckRaw, exists := d.values[deckField]
	if !exists {
		return fmt.Errorf("缺少存档字段 %s", deckField)
	}
	if _, err := parseDeck(deckRaw); err != nil {
		return err
	}
	return nil
}

func relicID(raw jsontext.Value) (int, error) {
	d, err := parse(raw)
	if err != nil {
		return 0, err
	}
	id, err := decode[int](d.values["DisplayId"])
	if err != nil {
		return 0, fmt.Errorf("遗物缺少 DisplayId")
	}
	return id, nil
}

func Read(path string) (string, []byte, error) {
	path, err := filepath.Abs(path)
	if err != nil || strings.ToLower(filepath.Ext(path)) != ".json" {
		return "", nil, fmt.Errorf("请选择 .json 存档文件")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxSize {
		return "", nil, fmt.Errorf("存档不是普通文件，或超过 16 MB")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	d, err := parse(raw)
	if err != nil {
		return "", nil, err
	}
	if err = validate(d); err != nil {
		return "", nil, err
	}
	return path, raw, nil
}

func Load(path string) (*Snapshot, error) {
	path, raw, err := Read(path)
	if err != nil {
		return nil, err
	}
	d, _ := parse(raw)
	result := &Snapshot{Path: path, Hash: Hash(raw), Version: string(d.values["Version"]), Fields: map[string]string{}, Relics: []HeldRelic{}}
	for _, field := range Fields {
		result.Fields[field.Key] = string(d.values[field.Key])
	}
	result.Equipped, _ = decode[[]int](d.values["AllXcp"])
	result.Warehouse, _ = decode[[]int](d.values["AllXcpRepository"])
	relics, _ := decode[[]jsontext.Value](d.values["RelicList"])
	for i, raw := range relics {
		id, _ := relicID(raw)
		result.Relics = append(result.Relics, HeldRelic{i, id})
	}
	deck, _ := parseDeck(d.values[deckField])
	result.Deck = deck.cards
	result.DeckRows = deck.rows
	return result, nil
}

func counts(ids []int) map[int]int {
	result := map[int]int{}
	for _, id := range ids {
		result[id]++
	}
	return result
}

func Build(raw []byte, request Request, catalog *Catalog) ([]byte, *Preview, error) {
	if request.Hash != Hash(raw) {
		return nil, nil, fmt.Errorf("存档已被游戏、Steam 或其他窗口修改，请重新加载")
	}
	d, err := parse(raw)
	if err != nil {
		return nil, nil, err
	}
	if err = validate(d); err != nil {
		return nil, nil, err
	}
	deckBefore, _ := parseDeck(d.values[deckField])
	numbers, labels := map[string]string{}, map[string]string{}
	for _, field := range Fields {
		numbers[field.Key], labels[field.Key] = string(d.values[field.Key]), field.Label
		if input, exists := request.Fields[field.Key]; exists {
			numbers[field.Key], err = number(input, field.Integer)
			if err != nil {
				return nil, nil, fmt.Errorf("%s：%w", field.Label, err)
			}
		}
	}
	for key := range request.Fields {
		if _, exists := numbers[key]; !exists {
			return nil, nil, fmt.Errorf("禁止修改字段 %s", key)
		}
	}
	intValue := func(key string) int { n, _ := strconv.Atoi(numbers[key]); return n }
	_, hpChanged := request.Fields["Hp"]
	_, maxChanged := request.Fields["HpMax"]
	if (hpChanged || maxChanged) && (intValue("HpMax") < 1 || intValue("Hp") > intValue("HpMax")) {
		return nil, nil, fmt.Errorf("最大血量至少为 1，当前血量不能超过最大血量")
	}
	if _, exists := request.Fields["XiaoChouRepositorySlot"]; exists && intValue("XiaoChouRepositorySlot") > 4 {
		return nil, nil, fmt.Errorf("原版仓库最多 4 个；已有超限值只能保留，不能新设超限容量")
	}
	oldEquipped, _ := decode[[]int](d.values["AllXcp"])
	oldWarehouse, _ := decode[[]int](d.values["AllXcpRepository"])
	equipped, warehouse := request.Equipped, request.Warehouse
	if equipped == nil {
		equipped = oldEquipped
	}
	if warehouse == nil {
		warehouse = oldWarehouse
	}
	if len(equipped) > 1024 || len(warehouse) > 1024 {
		return nil, nil, fmt.Errorf("灵俑列表过长")
	}
	oldCount := counts(append(append([]int{}, oldEquipped...), oldWarehouse...))
	newCount := counts(append(append([]int{}, equipped...), warehouse...))
	for id, n := range newCount {
		if _, known := catalog.Lingyong[id]; !known && n > oldCount[id] {
			return nil, nil, fmt.Errorf("禁止新增未知、BOSS 或角色技能灵俑：%d", id)
		}
		if n > max(1, oldCount[id]) {
			return nil, nil, fmt.Errorf("已持有灵俑 ID %d，不支持新增同 ID 副本", id)
		}
	}
	oldRelics, _ := decode[[]jsontext.Value](d.values["RelicList"])
	oldIDs, newIDs := []int{}, []int{}
	for _, item := range oldRelics {
		id, _ := relicID(item)
		oldIDs = append(oldIDs, id)
	}
	choices := request.Relics
	if choices == nil {
		choices = []RelicChoice{}
		for i := range oldRelics {
			choices = append(choices, RelicChoice{Ref: &i})
		}
	}
	relicTokens, used := []string{}, map[int]bool{}
	for _, choice := range choices {
		if choice.Ref != nil {
			i := *choice.Ref
			if choice.ID != nil || i < 0 || i >= len(oldRelics) || used[i] {
				return nil, nil, fmt.Errorf("遗物引用重复或越界")
			}
			used[i] = true
			relicTokens = append(relicTokens, string(oldRelics[i]))
			newIDs = append(newIDs, oldIDs[i])
		} else {
			if choice.ID == nil {
				return nil, nil, fmt.Errorf("遗物条目缺少 ID")
			}
			id := *choice.ID
			if _, known := catalog.Relics[id]; !known {
				return nil, nil, fmt.Errorf("不能新增未知遗物 %d", id)
			}
			relicTokens = append(relicTokens, fmt.Sprintf(`{"DisplayId":%d,"RoundCount":0,"PassNodeCount":0,"activationCount":0}`, id))
			newIDs = append(newIDs, id)
		}
	}
	oldRC, newRC := counts(oldIDs), counts(newIDs)
	slotChange := 0
	for id, n := range oldRC {
		slotChange -= catalog.SlotDelta(id) * n
	}
	for id, n := range newRC {
		slotChange += catalog.SlotDelta(id) * n
	}
	slots := intValue("LingYongSlotNum") + slotChange
	if slots < 0 || slots > 2147483647 {
		return nil, nil, fmt.Errorf("供台变动后的累计槽位超出范围")
	}
	numbers["LingYongSlotNum"] = strconv.Itoa(slots)
	if len(equipped) > min(slots, 8) {
		return nil, nil, fmt.Errorf("上场灵俑超出可用槽位，请先移入仓库或移除")
	}
	if len(warehouse) > intValue("XiaoChouRepositorySlot") {
		return nil, nil, fmt.Errorf("灵俑仓库超出容量，请先移出或移除")
	}
	changes := map[string]string{}
	for key, value := range numbers {
		if !equalNumber(value, string(d.values[key])) {
			changes[key] = value
		}
	}
	if !bytes.Equal([]byte(marshal(equipped)), []byte(marshal(oldEquipped))) {
		changes["AllXcp"] = marshal(equipped)
	}
	if !bytes.Equal([]byte(marshal(warehouse)), []byte(marshal(oldWarehouse))) {
		changes["AllXcpRepository"] = marshal(warehouse)
	}
	deckChanged := []string{}
	if request.Deck != nil {
		deckValues, err := normalizeDeckRequest(*request.Deck, deckBefore, catalog)
		if err != nil {
			return nil, nil, err
		}
		deckText, changed, err := buildDeck(deckBefore, deckValues)
		if err != nil {
			return nil, nil, err
		}
		deckChanged = changed
		if deckText != string(d.values[deckField]) {
			changes[deckField] = deckText
		}
	}
	growth, _ := parse(d.values["Xcp"])
	growthTokens := []string{}
	removed, derivedAdded := false, false
	for _, key := range growth.order {
		id, parseErr := strconv.Atoi(key)
		if parseErr == nil && oldCount[id] > 0 && newCount[id] == 0 {
			removed = true
			continue
		}
		growthTokens = append(growthTokens, marshal(key)+":"+string(growth.values[key]))
	}
	for _, id := range append(append([]int{}, equipped...), warehouse...) {
		key := strconv.Itoa(id)
		if oldCount[id] == 0 && newCount[id] > 0 {
			derivedAdded = derivedAdded || catalog.Lingyong[id].Derived
			if _, exists := growth.values[key]; !exists {
				growthTokens = append(growthTokens, marshal(key)+":"+fmt.Sprintf(`{"id":%d,"roundCount":0,"permanentAddFan":0.0,"permanentAddBaseScore":0.0,"accPermanentMultiIndependent":0.0,"addUpTimes":0,"addUpTimes2":0,"addUpTimes3":0,"addUpTimes4":0,"mark":0}`, id))
			}
		}
	}
	newGrowth := "{" + strings.Join(growthTokens, ",") + "}"
	// Compare serialized raw records, not float64 values.
	beforeGrowth, _ := decode[map[string]jsontext.Value](d.values["Xcp"])
	afterGrowth, _ := decode[map[string]jsontext.Value]([]byte(newGrowth))
	if marshal(beforeGrowth) != marshal(afterGrowth) {
		changes["Xcp"] = newGrowth
	}
	newRelicText := "[" + strings.Join(relicTokens, ",") + "]"
	if marshal(oldRelics) != marshal(jsontext.Value(newRelicText)) {
		changes["RelicList"] = newRelicText
	}
	updated := d.replace(changes)
	after, err := parse(updated)
	if err != nil {
		return nil, nil, err
	}
	if err = validate(after); err != nil {
		return nil, nil, err
	}
	for _, key := range d.order {
		if _, touched := changes[key]; !touched && string(after.values[key]) != string(d.values[key]) {
			return nil, nil, fmt.Errorf("内部校验失败：非目标字段 %s 有变化", key)
		}
	}
	preview := &Preview{Changes: []Change{}, Warnings: []string{}, SlotDelta: slotChange, Slots: slots, PreviewHash: Hash(updated)}
	for _, key := range d.order {
		if _, touched := changes[key]; !touched {
			continue
		}
		before, value := string(d.values[key]), changes[key]
		label := labels[key]
		if key == "AllXcp" || key == "AllXcpRepository" {
			label = map[string]string{"AllXcp": "上场灵俑", "AllXcpRepository": "灵俑仓库"}[key]
			list, _ := decode[[]int](d.values[key])
			next, _ := decode[[]int]([]byte(changes[key]))
			before, value = itemNames(list, catalog.Lingyong), itemNames(next, catalog.Lingyong)
		}
		if key == "Xcp" {
			label, before, value = "灵俑成长记录", fmt.Sprintf("%d 条", len(growth.values)), fmt.Sprintf("%d 条，未修改记录原文保留", len(afterGrowth))
		}
		if key == "RelicList" {
			removedIDs, addedIDs := []int{}, []int{}
			for index, id := range oldIDs {
				if !used[index] {
					removedIDs = append(removedIDs, id)
				}
			}
			for _, choice := range choices {
				if choice.ID != nil {
					addedIDs = append(addedIDs, *choice.ID)
				}
			}
			label, before, value = "持有遗物", fmt.Sprintf("%d 个", len(oldIDs)), fmt.Sprintf("%d 个", len(newIDs))
			if len(removedIDs) > 0 {
				before += "；移除：" + itemNames(removedIDs, catalog.Relics)
			}
			if len(addedIDs) > 0 {
				value += "；新增：" + itemNames(addedIDs, catalog.Relics)
			}
			if len(removedIDs) == 0 && len(addedIDs) == 0 {
				value += "（顺序调整，计数保留）"
			}
		}
		if key == deckField {
			label = "牌库"
			before = fmt.Sprintf("%d 条记录", deckBefore.rows)
			value = "34 条记录"
			if len(deckChanged) > 0 {
				shown := deckChanged
				if len(shown) > 8 {
					shown = shown[:8]
				}
				value += "；修改：" + strings.Join(shown, "、")
				if len(deckChanged) > len(shown) {
					value += fmt.Sprintf(" 等 %d 种", len(deckChanged))
				}
			} else {
				value += "（规范化）"
			}
		}
		preview.Changes = append(preview.Changes, Change{key, label, before, value})
	}
	if _, changed := changes["RelicList"]; changed {
		preview.Warnings = append(preview.Warnings, "离线编辑不执行遗物拾取事件，不随机加诅咒、不改牌组、不销毁灵俑。仅联动明确的灵俑槽增减，其他拾取时数值变化不模拟。")
	}
	if removed {
		preview.Warnings = append(preview.Warnings, "移除或替换会删除被移除灵俑的成长记录；上场与仓库间移动则保留成长。")
	}
	if derivedAdded {
		preview.Warnings = append(preview.Warnings, "你主动添加了衍生灵俑，仅改变当前持有物品，不加入发现或解锁列表。")
	}
	return updated, preview, nil
}

func itemNames(ids []int, items map[int]Item) string {
	names := []string{}
	for _, id := range ids {
		name := items[id].Name
		if name == "" {
			name = fmt.Sprintf("ID %d", id)
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "空"
	}
	return strings.Join(names, "、")
}
