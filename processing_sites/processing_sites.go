package processing_sites

import (
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

func ClearDateTimes(inp string) string {
	var sizes = []int{11, 10, 8, 5}
	var timeFormats = map[int][]string{
		5: {
			"15:04",
		},
		8: {
			"15:04:05",
		},
		10: {
			"2006-01-02",
			"2006/01/02",
			"02.01.2006",
			"2006.01.02",
		},
		11: {
			"02-Jan-2006",
			"Jan-02-2006",
			"Jan/02/2006",
		},
	}

	str := []rune(inp)
	var ret []rune
	for i := 0; i < len(str); {
		flag := false
		for _, sz := range sizes {
			if i+sz <= len(str) {
				substr := inp[i : i+sz]
				for _, format := range timeFormats[sz] {
					_, err := time.Parse(format, substr)
					if err == nil {
						flag = true
						i += sz
						break
					}
				}
				if flag {
					break
				}
			}
		}
		if !flag {
			ret = append(ret, str[i])
			i++
		}
	}
	return string(ret)
}

func IsTextTag(tag string) bool {
	var textTags = []string{"p", "span", "div", "a", "strong", "em", "i", "b", "u", "blockquote", "title"}
	tag = strings.ToLower(tag)
	for _, textTag := range textTags {
		if tag == textTag {
			return true
		}
	}
	return false
}

func GetHtml(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	html_text := string(body)

	return html_text, err
}

func GetOnlyText(url string) (string, error) {
	html_text_string, err := GetHtml(url)
	if err != nil {
		return "", err
	}
	html_text := []rune(html_text_string)

	var tags []string
	var data string

	for i := 0; i < len(html_text); {
		if html_text[i] == '<' {
			i++
			var tag []rune
			for ; i < len(html_text) && html_text[i] != '>' && html_text[i] != ' '; i++ {
				tag = append(tag, html_text[i])
			}
			for ; i < len(html_text) && html_text[i] != '>'; i++ {
			}
			i++
			if len(tag) > 0 {
				if tag[0] == '/' {
					tags = tags[:len(tags)-1]
				} else {
					tags = append(tags, string(tag))
				}
			}
		} else if len(tags) > 0 && IsTextTag(tags[len(tags)-1]) {
			var new_data []rune
			for ; i < len(html_text) && html_text[i] != '<'; i++ {
				if !unicode.IsControl(html_text[i]) &&
					(html_text[i] != ' ' || (html_text[i] == ' ' && len(new_data) > 0 && new_data[len(new_data)-1] != ' ')) {
					new_data = append(new_data, html_text[i])
				}
			}
			if len(new_data) > 0 && (len(new_data) > 1 || new_data[0] != ' ') {
				data += ClearDateTimes(string(new_data)) + "\n"
			}
		} else {
			i++
		}
	}

	return string(data), nil
}

func GetDifferences(data1, data2 string) (string, string) {
	pref := 0
	for pref < min(len(data1), len(data2)) && data1[pref] == data2[pref] {
		pref++
	}
	data1 = data1[pref:]
	data2 = data2[pref:]
	suf := 0
	for suf < min(len(data1), len(data2)) && data1[len(data1)-1-suf] == data2[len(data2)-1-suf] {
		suf++
	}
	return data1[:len(data1)-suf], data2[:len(data2)-suf]
}
