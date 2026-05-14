package flow

var choseong = []rune("ㄱㄲㄴㄷㄸㄹㅁㅂㅃㅅㅆㅇㅈㅉㅊㅋㅌㅍㅎ")
var jungseong = []rune("ㅏㅐㅑㅒㅓㅔㅕㅖㅗㅘㅙㅚㅛㅜㅝㅞㅟㅠㅡㅢㅣ")
var jongseong = []rune{0, 'ㄱ', 'ㄲ', 'ㄳ', 'ㄴ', 'ㄵ', 'ㄶ', 'ㄷ', 'ㄹ', 'ㄺ', 'ㄻ', 'ㄼ', 'ㄽ', 'ㄾ', 'ㄿ', 'ㅀ', 'ㅁ', 'ㅂ', 'ㅄ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}

var keyToJamo = map[rune]rune{
	'r': 'ㄱ', 'R': 'ㄲ', 's': 'ㄴ', 'e': 'ㄷ', 'E': 'ㄸ', 'f': 'ㄹ', 'a': 'ㅁ', 'q': 'ㅂ', 'Q': 'ㅃ',
	't': 'ㅅ', 'T': 'ㅆ', 'd': 'ㅇ', 'w': 'ㅈ', 'W': 'ㅉ', 'c': 'ㅊ', 'z': 'ㅋ', 'x': 'ㅌ', 'v': 'ㅍ', 'g': 'ㅎ',
	'k': 'ㅏ', 'o': 'ㅐ', 'i': 'ㅑ', 'O': 'ㅒ', 'j': 'ㅓ', 'p': 'ㅔ', 'u': 'ㅕ', 'P': 'ㅖ',
	'h': 'ㅗ', 'y': 'ㅛ', 'n': 'ㅜ', 'b': 'ㅠ', 'm': 'ㅡ', 'l': 'ㅣ',
}

var choIndex = indexRunes(choseong)
var jungIndex = indexRunes(jungseong)
var jongIndex = indexRunes(jongseong)

func KoreanTwoSetDisplayLength(keys string) int {
	return len([]rune(ComposeKoreanTwoSet(keys)))
}

func KoreanTwoSetChordedDisplayLength(keys string) int {
	return len([]rune(ComposeKoreanTwoSetChorded(keys)))
}

func ComposeKoreanTwoSet(keys string) string {
	output := make([]rune, 0, len(keys))
	jamo := make([]rune, 0, len(keys))

	flushJamo := func() {
		if len(jamo) == 0 {
			return
		}
		output = append(output, composeJamo(jamo)...)
		jamo = jamo[:0]
	}

	for _, key := range keys {
		mapped, ok := keyToJamo[key]
		if !ok {
			flushJamo()
			output = append(output, key)
			continue
		}
		jamo = append(jamo, mapped)
	}
	flushJamo()

	return string(output)
}

func ComposeKoreanTwoSetChorded(keys string) string {
	output := make([]rune, 0, len(keys))
	jamo := make([]rune, 0, len(keys))

	flushJamo := func() {
		if len(jamo) == 0 {
			return
		}
		output = append(output, composeChordedJamo(jamo)...)
		jamo = jamo[:0]
	}

	for _, key := range keys {
		mapped, ok := keyToJamo[key]
		if !ok {
			flushJamo()
			output = append(output, key)
			continue
		}
		jamo = append(jamo, mapped)
	}
	flushJamo()

	return string(output)
}

func composeJamo(input []rune) []rune {
	output := []rune{}
	for i := 0; i < len(input); {
		if i+1 < len(input) && isConsonant(input[i]) && isVowel(input[i+1]) {
			cho := input[i]
			jung := input[i+1]
			i += 2

			if i < len(input) && isVowel(input[i]) {
				if combined, ok := combineVowel(jung, input[i]); ok {
					jung = combined
					i++
				}
			}

			var jong rune
			if i < len(input) && isConsonant(input[i]) {
				if i+1 < len(input) && isVowel(input[i+1]) {
					output = append(output, composeSyllable(cho, jung, 0))
					continue
				}
				jong = input[i]
				i++
				if i < len(input) && isConsonant(input[i]) {
					if combined, ok := combineFinal(jong, input[i]); ok {
						jong = combined
						i++
					}
				}
			}

			output = append(output, composeSyllable(cho, jung, jong))
			continue
		}

		output = append(output, input[i])
		i++
	}
	return output
}

func composeChordedJamo(input []rune) []rune {
	output := []rune{}
	for i := 0; i < len(input); {
		if i+1 < len(input) && isConsonant(input[i]) && isVowel(input[i+1]) {
			consumed, syllable := consumeOrderedSyllable(input, i)
			output = append(output, syllable)
			i += consumed
			continue
		}

		if i+1 < len(input) && isVowel(input[i]) && isConsonant(input[i+1]) {
			output = append(output, composeSyllable(input[i+1], input[i], 0))
			i += 2
			continue
		}

		output = append(output, input[i])
		i++
	}
	return output
}

func consumeOrderedSyllable(input []rune, start int) (int, rune) {
	cho := input[start]
	jung := input[start+1]
	i := start + 2

	if i < len(input) && isVowel(input[i]) {
		if combined, ok := combineVowel(jung, input[i]); ok {
			jung = combined
			i++
		}
	}

	var jong rune
	if i < len(input) && isConsonant(input[i]) {
		if i+1 < len(input) && isVowel(input[i+1]) {
			return i - start, composeSyllable(cho, jung, 0)
		}
		jong = input[i]
		i++
		if i < len(input) && isConsonant(input[i]) {
			if combined, ok := combineFinal(jong, input[i]); ok {
				jong = combined
				i++
			}
		}
	}

	return i - start, composeSyllable(cho, jung, jong)
}

func composeSyllable(cho, jung, jong rune) rune {
	return rune(0xAC00 + choIndex[cho]*21*28 + jungIndex[jung]*28 + jongIndex[jong])
}

func isConsonant(value rune) bool {
	_, ok := choIndex[value]
	return ok
}

func isVowel(value rune) bool {
	_, ok := jungIndex[value]
	return ok
}

func combineVowel(left, right rune) (rune, bool) {
	pairs := map[[2]rune]rune{
		{'ㅗ', 'ㅏ'}: 'ㅘ', {'ㅗ', 'ㅐ'}: 'ㅙ', {'ㅗ', 'ㅣ'}: 'ㅚ',
		{'ㅜ', 'ㅓ'}: 'ㅝ', {'ㅜ', 'ㅔ'}: 'ㅞ', {'ㅜ', 'ㅣ'}: 'ㅟ',
		{'ㅡ', 'ㅣ'}: 'ㅢ',
	}
	value, ok := pairs[[2]rune{left, right}]
	return value, ok
}

func combineFinal(left, right rune) (rune, bool) {
	pairs := map[[2]rune]rune{
		{'ㄱ', 'ㅅ'}: 'ㄳ', {'ㄴ', 'ㅈ'}: 'ㄵ', {'ㄴ', 'ㅎ'}: 'ㄶ',
		{'ㄹ', 'ㄱ'}: 'ㄺ', {'ㄹ', 'ㅁ'}: 'ㄻ', {'ㄹ', 'ㅂ'}: 'ㄼ', {'ㄹ', 'ㅅ'}: 'ㄽ',
		{'ㄹ', 'ㅌ'}: 'ㄾ', {'ㄹ', 'ㅍ'}: 'ㄿ', {'ㄹ', 'ㅎ'}: 'ㅀ', {'ㅂ', 'ㅅ'}: 'ㅄ',
	}
	value, ok := pairs[[2]rune{left, right}]
	return value, ok
}

func indexRunes(values []rune) map[rune]int {
	index := map[rune]int{}
	for i, value := range values {
		index[value] = i
	}
	return index
}
