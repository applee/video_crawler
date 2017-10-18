package rules

import (
	"regexp"
	"time"
)

var StartDate = time.Now().Format("2006-01-02")

var ReURL = regexp.MustCompile(".*/([a-zA-Z0-9_=]+)\\.html")

// tx detail URL: https://v.qq.com/detail/q/q8742fxhp6pj7zv.html
// tx play URL: https://v.qq.com/x/cover/q8742fxhp6pj7zv.html
// youku detail URL: http://v.youku.com/v_show/id_XMjk4ODAyMzIyOA==.html
// youku play URL: http://v.youku.com/v_show/id_XMjk4ODAyMzIyOA==.html
func cutURL(url string) string {
	result := ReURL.FindStringSubmatch(url)
	if len(result) == 2 {
		return result[1]
	}
	return url
}
