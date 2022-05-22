package backup

import (
	"strings"
)

func LegalAlias(alias string) bool {
	if alias == "" {
		return false
	}
	for b := range alias {
		if !legalInUrl(byte(b)) {
			return false
		}
	}
	return true
}

func MakeLegalAlias(text string) (string, bool) {
	//return url.PathEscape(text)
	if len(text) == 0 {
		return "%00", false
	}
	var i int
	for i = 0; i < len(text); i++ {
		if !legalInUrl(text[i]){
			break
		}
	}
	if i >= len(text) {
		//No illegal characters
		return text, true
	}
	escaped := strings.Builder{}
	escaped.WriteString(text[:i])
	legal := true
	for ; i < len(text); i++ {
		if legalInUrl(text[i]){
			escaped.WriteByte(text[i])
		} else {
			if text[i] != ' ' {
				legal = false
			}
			escaped.WriteByte('%')
			hi, lo := toHex(text[i])
			escaped.WriteByte(hi)
			escaped.WriteByte(lo)
		}
	}
	return escaped.String(), legal
}

func toHex(in byte) (hi byte, lo byte) {
	lo = in & 0x0F
	if lo < 0x0A {
		lo += '0'
	} else {
		lo += 'A' - 0x0A
	}
	hi = (in >> 4) & 0x0F
	if hi < 0x0A {
		hi += '0'
	} else {
		hi += 'A' - 0x0A
	}
	return hi, lo
}

func legalInUrl(char byte) bool {
	if char < '!' || 'z' < char {
		return false
	}
	switch char {
	case
		'"',
		'#',
		'%',
		'&',
		'/',
		':',
		';',
		'<',
		'=',
		'>',
		'?',
		'@',
		'[',
		'\\',
		']',
		'^',
		'`':
		return false
	}
	return true
}
