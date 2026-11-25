package editor

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

type Field struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Integer bool   `json:"integer"`
}

var Fields = []Field{
	{"Coin", "金币", false}, {"Hp", "血量", true}, {"HpMax", "最大血量", true},
	{"AccHpLost", "累计已损失血量", true}, {"InitSwapPaiTotal", "开局调换牌数", true},
	{"SwapPaiTurnTotal", "每回合摸牌数", true}, {"HuPaiSlotsCount", "和牌槽数量", true},
	{"LingYongSlotNum", "灵俑累计槽位", true}, {"XiaoChouRepositorySlot", "灵俑仓库容量", true},
}

var numberPattern = regexp.MustCompile(`^\+?(\d+)(?:\.(\d+))?(?:[eE]([+-]?\d+))?$`)

// Expand decimal notation exactly; neither JS Number nor float64 touches save values.
func number(input string, integer bool) (string, error) {
	if len(input) == 0 || len(input) > 80 {
		return "", fmt.Errorf("请输入有效数字（不能为负数）")
	}
	match := numberPattern.FindStringSubmatch(input)
	if match == nil {
		return "", fmt.Errorf("请输入非负数字，不支持 NaN 或无穷大")
	}
	exponent := 0
	var err error
	if match[3] != "" {
		exponent, err = strconv.Atoi(match[3])
		if err != nil || exponent < -80 || exponent > 80 {
			return "", fmt.Errorf("数字指数超出范围")
		}
	}
	digits := strings.TrimLeft(match[1]+match[2], "0")
	if digits == "" {
		return "0", nil
	}
	scale := len(match[2]) - exponent
	var normalized string
	if scale <= 0 {
		normalized = digits + strings.Repeat("0", -scale)
	} else {
		if len(digits) <= scale {
			digits = strings.Repeat("0", scale-len(digits)+1) + digits
		}
		normalized = digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
		normalized = strings.TrimRight(strings.TrimRight(normalized, "0"), ".")
	}
	if integer {
		value, err := strconv.ParseInt(normalized, 10, 32)
		if err != nil || value < 0 {
			return "", fmt.Errorf("必须是 0 至 2147483647 的整数")
		}
	} else {
		value, _ := new(big.Rat).SetString(normalized)
		limit, _ := new(big.Rat).SetString("340282346638528860000000000000000000000")
		if value == nil || value.Cmp(limit) > 0 {
			return "", fmt.Errorf("金币超出游戏 float 数值范围")
		}
	}
	return normalized, nil
}

func equalNumber(a, b string) bool {
	x, ok := new(big.Rat).SetString(a)
	if !ok {
		return false
	}
	y, ok := new(big.Rat).SetString(b)
	return ok && x.Cmp(y) == 0
}
