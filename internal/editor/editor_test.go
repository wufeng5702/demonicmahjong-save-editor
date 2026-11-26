package editor

import (
	"bytes"
	"encoding/json/jsontext"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func fixture() []byte {
	return []byte(`{
  "Version":9,"Coin":801172.2,"Hp":3,"HpMax":3,"AccHpLost":32,
  "InitSwapPaiTotal":6,"SwapPaiTurnTotal":5,"HuPaiSlotsCount":29,
  "LingYongSlotNum":5,"XiaoChouRepositorySlot":4,
  "WorkbenchPaiMianBindList":[],
  "AllXcp":[1,2],"AllXcpRepository":[],
  "Xcp":{"1":{"id":1,"permanentAddFan":0.0}, "2": {"id":2,"accPermanentMultiIndependent":5.279997,"future":79228162514264337593543950335.0},"99":{"id":99,"mark":7}},
  "RelicList":[{"DisplayId":77,"RoundCount":4,"activationCount":2,"future":{"exact":79228162514264337593543950335.0}}],
  "AccountGameData":{"achievements":{"MaxSumScore":79228162514264337593543950335.0,"MaxHuScore":161461040910412030.0}},
  "HadXiaoChouList":[1,98],"UnlockedLingYongList":[9,17],
  "HadRelicList":[77],"UnlockedRelicList":["QingLingGuan"],
  "MapHistoryPath":"keep me","AllFakeXcp":[20039]
}
`)
}

func catalogFixture() *Catalog {
	return &Catalog{Lingyong: map[int]Item{1: {ID: 1, Name: "万福娃娃"}, 2: {ID: 2, Name: "气筒娃娃"}, 14: {ID: 14, Name: "三头犬"}, 98: {ID: 98, Name: "雏凤", Derived: true}},
		Relics:  map[int]Item{77: {ID: 77, Name: "怪异供台", Desc: "你的灵俑槽+1（上限为8）。"}, 3: {ID: 3, Name: "莲台", Desc: "你的灵俑槽+1（上限为8）。"}, 188: {ID: 188, Name: "外婆的祝福", Tags: []string{"供台"}, Desc: "拾取时获得一个供台。"}},
		Pailing: map[int]Item{10027: {ID: 10027, Name: "鼠灵", Desc: "牌面数字-2。"}},
		Baopai:  map[int]Item{1: {ID: 1, Name: "红宝牌", Desc: "倍率+25%。"}}}
}

func req(raw []byte) Request { return Request{Hash: Hash(raw)} }

func TestNoOpPreservesEveryByte(t *testing.T) {
	for _, raw := range [][]byte{fixture(), append([]byte{239, 187, 191}, fixture()...)} {
		updated, preview, err := Build(raw, req(raw), catalogFixture())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, updated) || len(preview.Changes) != 0 {
			t.Fatal("no-op changed bytes")
		}
	}
}

func TestNumericsPreserveHugeDecimalsAndAllOtherBytes(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Fields = map[string]string{"Coin": "1000000.25", "Hp": "2"}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	expected := bytes.Replace(raw, []byte(`"Coin":801172.2`), []byte(`"Coin":1000000.25`), 1)
	expected = bytes.Replace(expected, []byte(`"Hp":3`), []byte(`"Hp":2`), 1)
	if !bytes.Equal(updated, expected) || len(preview.Changes) != 2 {
		t.Fatal("unrelated bytes or decimal literals changed")
	}
}

func TestAccountCurrencyReadsAndReplacesOnlyEditedBalances(t *testing.T) {
	raw := bytes.Replace(fixture(), []byte(`"AccountGameData":{"achievements":`), []byte(`"AccountGameData":{
    "currency" : {"cashTokenCount":7, "lowTokenCount" : 12, "premiumTokenCount":34, "future":79228162514264337593543950335.0},
    "achievements":`), 1)
	path := filepath.Join(t.TempDir(), "Slot0.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	save, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	const low = "AccountGameData.currency.lowTokenCount"
	const premium = "AccountGameData.currency.premiumTokenCount"
	if save.Fields[low] != "12" || save.Fields[premium] != "34" {
		t.Fatalf("unexpected balances: %q / %q", save.Fields[low], save.Fields[premium])
	}
	request := req(raw)
	request.Fields = map[string]string{premium: "0"}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	expected := bytes.Replace(raw, []byte(`"premiumTokenCount":34`), []byte(`"premiumTokenCount":0`), 1)
	if !bytes.Equal(updated, expected) || !slices.Equal(preview.Changes, []Change{{premium, "精魄", "34", "0"}}) {
		t.Fatalf("single balance edit changed unexpected content or preview: %+v", preview.Changes)
	}
	request.Fields[low] = "2147483647"
	updated, preview, err = Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	expected = bytes.Replace(expected, []byte(`"lowTokenCount" : 12`), []byte(`"lowTokenCount" : 2147483647`), 1)
	wantChanges := []Change{{low, "修为", "12", "2147483647"}, {premium, "精魄", "34", "0"}}
	if !bytes.Equal(updated, expected) || !slices.Equal(preview.Changes, wantChanges) {
		t.Fatalf("two balance edits changed unexpected content or preview: %+v", preview.Changes)
	}
	backup, err := AtomicSave(path, save.Hash, updated, func() (bool, error) { return false, nil })
	if err != nil {
		t.Fatal(err)
	}
	backedUp, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(backedUp, raw) {
		t.Fatalf("backup did not preserve the original bytes: %v", err)
	}
	save, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if save.Fields[low] != "2147483647" || save.Fields[premium] != "0" {
		t.Fatalf("unexpected balances after saving: %q / %q", save.Fields[low], save.Fields[premium])
	}
}

func TestAccountCurrencyUnavailableAndInvalidEdits(t *testing.T) {
	const low = "AccountGameData.currency.lowTokenCount"
	const premium = "AccountGameData.currency.premiumTokenCount"
	old := fixture()
	request := req(old)
	request.Fields = map[string]string{low: "1"}
	if _, _, err := Build(old, request, catalogFixture()); err == nil {
		t.Fatal("accepted editing a balance missing from an older save")
	}
	raw := bytes.Replace(old, []byte(`"AccountGameData":{"achievements":`), []byte(`"AccountGameData":{"currency":{"lowTokenCount":0,"premiumTokenCount":null},"achievements":`), 1)
	for key, inputs := range map[string][]string{
		low: {"-1", "12.5", "2147483648"}, premium: {"10"}, "AccountGameData.currency.cashTokenCount": {"10"},
	} {
		for _, input := range inputs {
			request := req(raw)
			request.Fields = map[string]string{key: input}
			if _, _, err := Build(raw, request, catalogFixture()); err == nil {
				t.Errorf("accepted %s = %q", key, input)
			}
		}
	}
	request = req(raw)
	request.Fields = map[string]string{low: "17"}
	updated, _, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(updated, bytes.Replace(raw, []byte(`"lowTokenCount":0`), []byte(`"lowTokenCount":17`), 1)) {
		t.Fatal("editing the available balance changed other content")
	}
}

func TestNumberValidation(t *testing.T) {
	for _, value := range []string{"-1", "NaN", "Infinity", "1e1000000", "0e-1000000", "1.5", "2147483648"} {
		if _, err := number(value, true); err == nil {
			t.Errorf("accepted integer %s", value)
		}
	}
	for input, want := range map[string]string{"1e5": "100000", "001.2500": "1.25", "0.0001": "0.0001", "20.0": "20", "0": "0"} {
		got, err := number(input, false)
		if err != nil || got != want {
			t.Errorf("%s got %s (%v), want %s", input, got, err, want)
		}
	}
	for input, want := range map[string]string{"-20": "-20.0", "+10.50": "10.5", "0": "0.0", "1e2": "100.0"} {
		got, err := deckScore(input)
		if err != nil || got != want {
			t.Errorf("deck score %s got %s (%v), want %s", input, got, err, want)
		}
	}
	for _, input := range []string{"NaN", "Infinity", "3.5e38", "1e1000"} {
		if _, err := deckScore(input); err == nil {
			t.Errorf("accepted invalid deck score %s", input)
		}
	}
}

func TestInvalidRequests(t *testing.T) {
	raw := fixture()
	tests := []Request{
		{Hash: "stale"},
		{Hash: Hash(raw), Fields: map[string]string{"UnlockedLingYongList": "1"}},
		{Hash: Hash(raw), Fields: map[string]string{"Hp": "4"}},
		{Hash: Hash(raw), Fields: map[string]string{"HpMax": "0"}},
		{Hash: Hash(raw), Fields: map[string]string{"XiaoChouRepositorySlot": "5"}},
		{Hash: Hash(raw), Equipped: []int{1, 2, 10001}},
		{Hash: Hash(raw), Equipped: []int{1, 2, 20039}},
		{Hash: Hash(raw), Equipped: []int{1, 1, 2}},
		{Hash: Hash(raw), Warehouse: []int{1}},
	}
	for i, request := range tests {
		if _, _, err := Build(raw, request, catalogFixture()); err == nil {
			t.Errorf("accepted invalid request %d", i)
		}
	}
}

func TestMovePreservesGrowthExactly(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Equipped = []int{1}
	request.Warehouse = []int{2}
	updated, _, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	if !bytes.Equal(before.values["Xcp"], after.values["Xcp"]) {
		t.Fatal("moving reset growth")
	}
}

func TestReplacementPreservesOtherGrowthAndOrphans(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Equipped = []int{14, 2}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	oldGrowth, _ := parse(before.values["Xcp"])
	newGrowth, _ := parse(after.values["Xcp"])
	for _, key := range []string{"2", "99"} {
		if !bytes.Equal(oldGrowth.values[key], newGrowth.values[key]) {
			t.Fatalf("changed existing growth %s", key)
		}
	}
	if _, exists := newGrowth.values["1"]; exists {
		t.Fatal("removed growth retained")
	}
	if !bytes.Contains(newGrowth.values["14"], []byte(`"roundCount":0`)) || len(preview.Warnings) == 0 {
		t.Fatal("new record or warning missing")
	}
	for _, key := range []string{"HadXiaoChouList", "UnlockedLingYongList", "HadRelicList", "UnlockedRelicList"} {
		if !bytes.Equal(before.values[key], after.values[key]) {
			t.Fatalf("mutated collection %s", key)
		}
	}
}

func TestDerivedIsInventoryOnly(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Warehouse = []int{98}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	if len(preview.Warnings) == 0 || !bytes.Equal(before.values["UnlockedLingYongList"], after.values["UnlockedLingYongList"]) {
		t.Fatal("derived item touched shop pool")
	}
}

func TestRelicSlotDeltaAndCounterPreservation(t *testing.T) {
	raw := fixture()
	request := req(raw)
	zero, three := 0, 3
	request.Relics = []RelicChoice{{Ref: &zero}, {ID: &three}}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	if preview.Slots != 6 || preview.SlotDelta != 1 {
		t.Fatal("slot delta incorrect")
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	old, _ := decode[[]jsontext.Value](before.values["RelicList"])
	next, _ := decode[[]jsontext.Value](after.values["RelicList"])
	if !bytes.Equal(old[0], next[0]) {
		t.Fatal("existing relic counters changed")
	}
	if catalogFixture().SlotDelta(188) != 0 {
		t.Fatal("guessed slot bonus from tag")
	}
}

func TestRelicBatchAddAllowsDuplicateItems(t *testing.T) {
	raw := fixture()
	zero, three, grandmother := 0, 3, 188
	request := req(raw)
	request.Relics = []RelicChoice{{Ref: &zero}, {ID: &three}, {ID: &grandmother}}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	after, _ := parse(updated)
	rows, _ := decode[[]jsontext.Value](after.values["RelicList"])
	if len(rows) != 3 || preview.SlotDelta != 1 {
		t.Fatal("batch add or altar delta incorrect")
	}
	if !strings.Contains(preview.Changes[0].After, "6") {
		t.Fatal("slot preview missing")
	}
	request.Relics = append(request.Relics, RelicChoice{ID: &three})
	updated, preview, err = Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal("duplicate relic rejected:", err)
	}
	after, _ = parse(updated)
	rows, _ = decode[[]jsontext.Value](after.values["RelicList"])
	if len(rows) != 4 || preview.SlotDelta != 2 {
		t.Fatal("duplicate altar did not add its own slot")
	}
	request.Relics = []RelicChoice{{Ref: &zero}, {Ref: &zero}}
	if _, _, err := Build(raw, request, catalogFixture()); err == nil {
		t.Fatal("accepted duplicated original record")
	}
}

func TestManyDuplicateRelicsHaveNoGameQuantityCap(t *testing.T) {
	raw := fixture()
	request := req(raw)
	id := 188
	request.Relics = []RelicChoice{}
	for i := 0; i < 4200; i++ {
		request.Relics = append(request.Relics, RelicChoice{ID: &id})
	}
	updated, _, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := parse(updated)
	rows, _ := decode[[]jsontext.Value](doc.values["RelicList"])
	if len(rows) != 4200 {
		t.Fatal("relic quantity was capped")
	}
}

func deckValuesFromRaw(t *testing.T, raw []byte) []DeckValue {
	t.Helper()
	doc, err := parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	state, err := parseDeck(doc.values[deckField])
	if err != nil {
		t.Fatal(err)
	}
	values := make([]DeckValue, 0, len(state.cards))
	for _, card := range state.cards {
		values = append(values, card.DeckValue)
	}
	return values
}

func TestCanonicalDeckOrderAndHonorGapModel(t *testing.T) {
	if len(canonicalTiles) != 34 {
		t.Fatalf("got %d canonical tiles", len(canonicalTiles))
	}
	for i := 0; i < 9; i++ {
		if canonicalTiles[i].Key != "wan"+strconv.Itoa(i+1) || canonicalTiles[i+9].Key != "tong"+strconv.Itoa(i+1) || canonicalTiles[i+18].Key != "suo"+strconv.Itoa(i+1) {
			t.Fatal("numbered tiles are not ordered as 1-9 wan/tong/suo")
		}
	}
	wantHonors := []string{"dong", "nan", "xi", "bei", "zhong", "fa", "bai"}
	for i, key := range wantHonors {
		if canonicalTiles[27+i].Key != key {
			t.Fatalf("honor %d: got %s want %s", i, canonicalTiles[27+i].Key, key)
		}
	}
}

func TestDeckLoadsEmpty33And34Rows(t *testing.T) {
	empty, err := parseDeck(jsontext.Value(`[]`))
	if err != nil || empty.rows != 0 || len(empty.cards) != 34 {
		t.Fatalf("empty deck: state=%+v err=%v", empty, err)
	}
	values := make(map[string]DeckValue, 34)
	for _, card := range empty.cards {
		values[card.Key] = card.DeckValue
	}
	full, _, err := buildDeck(empty, values)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := decode[[]jsontext.Value](jsontext.Value(full))
	if err != nil || len(rows) != 34 {
		t.Fatalf("full deck: rows=%d err=%v", len(rows), err)
	}
	for count, raw := range map[int]string{33: marshal(rows[:33]), 34: full} {
		state, err := parseDeck(jsontext.Value(raw))
		if err != nil || state.rows != count || len(state.cards) != 34 {
			t.Fatalf("%d-row deck: state=%+v err=%v", count, state, err)
		}
	}
}

func TestDeckEditNormalizesTo34AndPreservesOtherRootValues(t *testing.T) {
	raw := bytes.Replace(fixture(), []byte(`"WorkbenchPaiMianBindList":[]`), []byte(`"WorkbenchPaiMianBindList":[{"PaiMian":{"huaSe":1,"number":1,"feng":0,"jian":0},"DeltaBaseScore":0.0,"BaoId":0,"LingId":0,"future":{"huge":79228162514264337593543950335.0}}]`), 1)
	values := deckValuesFromRaw(t, raw)
	values[0].Score = "-20"
	values[0].BaoID = 1
	values[0].LingID = 10027
	request := req(raw)
	request.Deck = &values
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	state, err := parseDeck(after.values[deckField])
	if err != nil {
		t.Fatal(err)
	}
	if state.rows != 34 || len(state.byKey) != 34 {
		t.Fatalf("deck was not normalized: rows=%d unique=%d", state.rows, len(state.byKey))
	}
	wan1 := state.byKey["wan1"]
	if wan1.value.Score != "-20.0" || wan1.value.BaoID != 1 || wan1.value.LingID != 10027 {
		t.Fatalf("wan1 values not updated: %+v", wan1.value)
	}
	if _, exists := wan1.doc.values["future"]; !exists {
		t.Fatal("existing deck row unknown field was discarded")
	}
	if !bytes.Equal(before.values["AccountGameData"], after.values["AccountGameData"]) {
		t.Fatal("deck edit changed an unrelated root field")
	}
	if len(preview.Changes) != 1 || preview.Changes[0].Field != deckField || !strings.Contains(preview.Changes[0].After, "1万") {
		t.Fatalf("deck preview missing: %+v", preview.Changes)
	}
}

func TestDeckUntouchedPreservesSparseBytes(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Fields = map[string]string{"Coin": "123.5"}
	updated, _, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	if !bytes.Equal(before.values[deckField], after.values[deckField]) {
		t.Fatal("unmodified deck was normalized")
	}
}

func TestDeckRejectsMissingDuplicateAndUnknownEffects(t *testing.T) {
	raw := fixture()
	values := deckValuesFromRaw(t, raw)
	missing := append([]DeckValue(nil), values[:33]...)
	request := req(raw)
	request.Deck = &missing
	if _, _, err := Build(raw, request, catalogFixture()); err == nil {
		t.Fatal("accepted a deck missing one tile")
	}
	duplicate := append([]DeckValue(nil), values...)
	duplicate[33].Key = duplicate[0].Key
	request.Deck = &duplicate
	if _, _, err := Build(raw, request, catalogFixture()); err == nil {
		t.Fatal("accepted duplicate tile")
	}
	unknown := append([]DeckValue(nil), values...)
	unknown[0].BaoID = 999999
	request.Deck = &unknown
	if _, _, err := Build(raw, request, catalogFixture()); err == nil {
		t.Fatal("accepted unknown new baopai")
	}
}

func TestSaveCopiesRemainUnchanged(t *testing.T) {
	dir := t.TempDir()
	for slot := 0; slot < 2; slot++ {
		raw := bytes.Replace(fixture(), []byte(`"XiaoChouRepositorySlot":4`), []byte(`"XiaoChouRepositorySlot":`+strconv.Itoa(2+slot*2)), 1)
		if err := os.WriteFile(filepath.Join(dir, "Slot"+strconv.Itoa(slot)+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := filepath.Glob(filepath.Join(dir, "Slot*.json"))
	if err != nil {
		t.Fatal(err)
	}
	cat := catalogFixture()
	for _, path := range paths {
		_, raw, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		next, _, err := Build(raw, req(raw), cat)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !bytes.Equal(raw, next) {
			t.Fatal("save-copy no-op changed bytes")
		}
		request := req(raw)
		request.Fields = map[string]string{"Coin": "1000000.25"}
		next, _, err = Build(raw, request, cat)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := parse(raw)
		after, _ := parse(next)
		for _, key := range before.order {
			if key != "Coin" && !bytes.Equal(before.values[key], after.values[key]) {
				t.Fatalf("changed %s", key)
			}
		}
		onDisk, _ := os.ReadFile(path)
		if !bytes.Equal(raw, onDisk) {
			t.Fatal("integration test modified a save copy")
		}
	}
}

func TestLimitsFiveThroughTwentyAndShrink(t *testing.T) {
	cat := catalogFixture()
	for id := 100; id < 110; id++ {
		cat.Lingyong[id] = Item{ID: id}
	}
	for _, slots := range []int{5, 6, 7, 8, 20} {
		raw := bytes.Replace(fixture(), []byte(`"LingYongSlotNum":5`), []byte(`"LingYongSlotNum":`+strconv.Itoa(slots)), 1)
		request := req(raw)
		for i := 0; i < min(slots, 8); i++ {
			request.Equipped = append(request.Equipped, 100+i)
		}
		if _, _, err := Build(raw, request, cat); err != nil {
			t.Fatalf("capacity %d: %v", slots, err)
		}
		request.Equipped = append(request.Equipped, 109)
		if _, _, err := Build(raw, request, cat); err == nil {
			t.Fatalf("accepted overflow at %d", slots)
		}
	}
	raw := fixture()
	request := req(raw)
	request.Fields = map[string]string{"LingYongSlotNum": "2"}
	request.Relics = []RelicChoice{}
	if _, _, err := Build(raw, request, cat); err == nil {
		t.Fatal("silently deleted overflow items on relic removal")
	}
}

func TestDuplicateJSONKeysRejected(t *testing.T) {
	if _, err := parse([]byte(`{"Coin":1,"Coin":2}`)); err == nil {
		t.Fatal("accepted duplicate keys")
	}
	if _, err := parse([]byte(`{"nested":{"key":1,"key":2}}`)); err == nil {
		t.Fatal("accepted nested duplicate keys")
	}
	if _, err := decode[Request](jsontext.Value(`{"path":"first","path":"second"}`)); err == nil {
		t.Fatal("encoding/json/v2 accepted duplicate request fields")
	}
	if _, err := parse([]byte(`{"a":1} garbage`)); err == nil {
		t.Fatal("accepted trailing garbage")
	}
	if _, err := parse([]byte{'{', '"', 'a', '"', ':', '"', 0xff, '"', '}'}); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
}

func TestAtomicSaveBackupConflictAndGameGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Slot0.json")
	raw := fixture()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	request := req(raw)
	request.Fields = map[string]string{"Coin": "123.5"}
	updated, _, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AtomicSave(path, Hash(raw), updated, func() (bool, error) { return true, nil }); err == nil {
		t.Fatal("wrote while game running")
	}
	if _, err := AtomicSave(path, "stale", updated, func() (bool, error) { return false, nil }); err == nil {
		t.Fatal("accepted stale file")
	}
	checks := 0
	if _, err := AtomicSave(path, Hash(raw), updated, func() (bool, error) { checks++; return checks > 1, nil }); err == nil {
		t.Fatal("ignored game startup during save")
	}
	onDisk, _ := os.ReadFile(path)
	if !bytes.Equal(onDisk, raw) {
		t.Fatal("failed save modified original")
	}
	backup, err := AtomicSave(path, Hash(raw), updated, func() (bool, error) { return false, nil })
	if err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(backup)
	next, _ := os.ReadFile(path)
	if !bytes.Equal(old, raw) || !bytes.Equal(next, updated) || !strings.Contains(backup, ".save-editor.") {
		t.Fatal("backup or write incorrect")
	}
}
