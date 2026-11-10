// Package country derives the country of a server from its name and provides
// the flag assets shown in the UI.
package country

import (
	"strings"
	"unicode"
)

// codeToName maps a two letter country code to a Russian display name for the
// countries that show up in proxy server lists most often.
var codeToName = map[string]string{
	"RU": "Россия", "UA": "Украина", "BY": "Беларусь", "KZ": "Казахстан",
	"KG": "Кыргызстан", "UZ": "Узбекистан", "AM": "Армения", "GE": "Грузия",
	"AZ": "Азербайджан", "MD": "Молдова", "TJ": "Таджикистан",
	"TM": "Туркменистан", "MN": "Монголия", "CN": "Китай", "HK": "Гонконг",
	"TW": "Тайвань", "JP": "Япония", "KR": "Южная Корея", "SG": "Сингапур",
	"MY": "Малайзия", "TH": "Таиланд", "VN": "Вьетнам", "ID": "Индонезия",
	"PH": "Филиппины", "IN": "Индия", "PK": "Пакистан", "IR": "Иран",
	"TR": "Турция", "AE": "ОАЭ", "SA": "Саудовская Аравия", "IL": "Израиль",
	"DE": "Германия", "NL": "Нидерланды", "FR": "Франция", "GB": "Великобритания",
	"UK": "Великобритания", "US": "США", "CA": "Канада", "MX": "Мексика",
	"BR": "Бразилия", "AR": "Аргентина", "CL": "Чили", "CO": "Колумбия",
	"PL": "Польша", "CZ": "Чехия", "SK": "Словакия", "AT": "Австрия",
	"CH": "Швейцария", "SE": "Швеция", "NO": "Норвегия", "FI": "Финляндия",
	"DK": "Дания", "IE": "Ирландия", "ES": "Испания", "PT": "Португалия",
	"IT": "Италия", "RO": "Румыния", "BG": "Болгария", "GR": "Греция",
	"LT": "Литва", "LV": "Латвия", "EE": "Эстония", "HU": "Венгрия",
	"UA2": "Украина", "IS": "Исландия", "LU": "Люксембург", "HR": "Хорватия",
	"RS": "Сербия", "SI": "Словения", "SK2": "Словакия", "AE2": "ОАЭ",
}

// nameToCode maps country names (English and Russian) to their code.
var nameToCode = map[string]string{
	"russia": "RU", "russian federation": "RU", "россия": "RU", "рф": "RU",
	"ukraine": "UA", "украина": "UA",
	"belarus": "BY", "беларусь": "BY",
	"kazakhstan": "KZ", "казахстан": "KZ",
	"germany": "DE", "germania": "DE", "германия": "DE", "деф": "DE",
	"netherlands": "NL", "holland": "NL", "нидерланды": "NL", "голландия": "NL",
	"france": "FR", "франция": "FR",
	"united kingdom": "GB", "england": "GB", "britain": "GB", "великобритания": "GB",
	"united states": "US", "usa": "US", "america": "US", "сша": "US",
	"canada": "CA", "канада": "CA",
	"finland": "FI", "финляндия": "FI",
	"sweden": "SE", "швеция": "SE",
	"norway": "NO", "норвегия": "NO",
	"denmark": "DK", "дания": "DK",
	"poland": "PL", "польша": "PL",
	"czech": "CZ", "czechia": "CZ", "чехия": "CZ",
	"spain": "ES", "испания": "ES",
	"portugal": "PT", "португалия": "PT",
	"italy": "IT", "италия": "IT",
	"japan": "JP", "япония": "JP",
	"singapore": "SG", "сингапур": "SG",
	"hong kong": "HK", "гонконг": "HK",
	"taiwan": "TW", "тайвань": "TW",
	"turkey": "TR", "turkiye": "TR", "турция": "TR",
	"india": "IN", "индия": "IN",
	"australia": "AU", "австралия": "AU",
	"argentina": "AR", "аргентина": "AR",
	"brazil": "BR", "бразилия": "BR",
	"mexico": "MX", "мексика": "MX",
	"latvia": "LV", "латвия": "LV",
	"lithuania": "LT", "литва": "LT",
	"estonia": "EE", "эстония": "EE",
	"romania": "RO", "румыния": "RO",
	"bulgaria": "BG", "болгария": "BG",
	"greece": "GR", "греция": "GR",
	"hungary": "HU", "венгрия": "HU",
	"austria": "AT", "австрия": "AT",
	"switzerland": "CH", "швейцария": "CH",
	"ireland": "IE", "ирландия": "IE",
	"iceland": "IS", "исландия": "IS",
}

var validCodes = map[string]bool{}

// isoCodes holds every valid ISO 3166-1 alpha-2 code, so a flag emoji is only
// accepted when it really is a country flag.
var isoCodes = map[string]bool{}

func init() {
	for code := range codeToName {
		validCodes[code] = true
	}
	for code := range nameToCode {
		validCodes[code] = true
	}
	for _, code := range strings.Fields(isoCodeList) {
		isoCodes[code] = true
	}
}

// isoCodeList is the full list of ISO 3166-1 alpha-2 country codes.
const isoCodeList = `AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ
CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW`

// Detect guesses the country of a server from its display name. It returns an
// empty code when nothing can be determined.
func Detect(name string) (code string, country string) {
	if c := fromEmoji(name); c != "" {
		return c, Name(c)
	}
	if c := fromCodeToken(name); c != "" {
		return c, Name(c)
	}
	if c := fromName(name); c != "" {
		return c, Name(c)
	}
	return "", ""
}

// fromEmoji converts a flag emoji into a country code.
func fromEmoji(name string) string {
	runes := []rune(name)
	for i := 0; i+1 < len(runes); i++ {
		a, b := runes[i], runes[i+1]
		if a >= 0x1F1E6 && a <= 0x1F1FF && b >= 0x1F1E6 && b <= 0x1F1FF {
			code := string([]rune{a - 0x1F1E6 + 'A', b - 0x1F1E6 + 'A'})
			if isoCodes[code] {
				return code
			}
		}
	}
	return ""
}

// fromCodeToken looks for a standalone two letter country code in the name.
func fromCodeToken(name string) string {
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, f := range fields {
		if len(f) != 2 {
			continue
		}
		upper := strings.ToUpper(f)
		if validCodes[upper] {
			return upper
		}
	}
	return ""
}

// fromName looks for a known country name inside the server name.
func fromName(name string) string {
	lower := strings.ToLower(name)
	for needle, code := range nameToCode {
		if len(needle) < 4 {
			continue
		}
		if strings.Contains(lower, needle) {
			return code
		}
	}
	return ""
}

// Name returns the Russian display name of a country code.
func Name(code string) string {
	if n, ok := codeToName[code]; ok {
		return n
	}
	return code
}

// Emoji returns the flag emoji for a country code.
func Emoji(code string) string {
	if len(code) != 2 || !isoCodes[strings.ToUpper(code)] {
		return "🏳️"
	}
	out := []rune{}
	for _, r := range strings.ToUpper(code) {
		out = append(out, r-'A'+0x1F1E6)
	}
	return string(out)
}

// FlagURL returns a URL of the country flag image, empty when unknown.
func FlagURL(code string) string {
	if len(code) != 2 || !isoCodes[strings.ToUpper(code)] {
		return ""
	}
	return "https://flagcdn.com/w40/" + strings.ToLower(code) + ".png"
}
