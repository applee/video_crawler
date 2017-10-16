package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/applee/go-common/slice"
	"github.com/henrylee2cn/pholcus/app/downloader/request"
	"github.com/henrylee2cn/pholcus/app/spider"
	"github.com/henrylee2cn/pholcus/common/goquery"
)

var (
	YoukuChannels = []string{
		"电影",
		"剧集",
		"综艺",
		"动漫",
		"少儿",
	}
	YoukuListUrl = "http://list.youku.com"
	CategoryRe   = regexp.MustCompile("^(.*?)\\d+\\.html.*$")
)

func init() {
	Youku.Register()
}

func YoukuNamespace(*spider.Spider) string {
	return "youku"
}

func YoukuSubNamespace(*spider.Spider, map[string]interface{}) string {
	return "videos"
}

var Youku = &spider.Spider{
	Name:            "优酷视频详情",
	Description:     "优酷视频详情[http://list.youku.com/category]",
	Namespace:       YoukuNamespace,
	SubNamespace:    YoukuSubNamespace,
	NotDefaultField: true,
	// Pausetime:    300,
	// Keyin:        KEYIN,
	EnableCookie: false,
	RuleTree: &spider.RuleTree{
		Root: func(ctx *spider.Context) {
			ctx.AddQueue(&request.Request{
				Url:  YoukuListUrl,
				Rule: "category",
			})
		},
		Trunk: map[string]*spider.Rule{
			"category": {
				ParseFunc: func(ctx *spider.Context) {
					query := ctx.GetDom()
					query.Find(".yk-filter-panel ul>li>a").Each(func(i int, s *goquery.Selection) {
						if index := slice.Index(len(YoukuChannels),
							func(i int) bool { return YoukuChannels[i] == s.Text() }); index >= 0 {
							if url, ok := s.Attr("href"); ok {
								ctx.AddQueue(&request.Request{
									Url:  YoukuListUrl + url,
									Rule: "pages",
									Temp: map[string]interface{}{
										"channel": index,
									},
								})
								time.Sleep(time.Minute * 5)
							}
						}
					})
				},
			},
			"pages": {
				AidFunc: func(ctx *spider.Context, aid map[string]interface{}) interface{} {
					url := aid["url"].(string)
					channel := ctx.GetTemp("channel", 0).(int)
					for loop := aid["loop"].([2]int); loop[0] <= loop[1]; loop[0]++ {
						ctx.AddQueue(&request.Request{
							Url:  CategoryRe.ReplaceAllString(url, fmt.Sprintf("${1}%d.html", loop[0])),
							Rule: aid["Rule"].(string),
							Temp: map[string]interface{}{
								"channel": channel,
							},
						})
						time.Sleep(time.Second * 20)
					}
					return nil
				},
				ParseFunc: func(ctx *spider.Context) {
					query := ctx.GetDom()
					lastPage := query.Find(".yk-pages .next").Prev().Children().First()
					if lastPage != nil {
						url, ok := lastPage.Attr("href")
						if !ok {
							return
						}
						total, err := strconv.Atoi(lastPage.Text())
						if err != nil {
							return
						}
						ctx.Aid(map[string]interface{}{
							"loop": [2]int{2, total},
							"Rule": "list",
							"url":  parseUrl(url),
						})
					}
					ctx.Parse("list")
				},
			},
			"list": {
				ParseFunc: func(ctx *spider.Context) {
					query := ctx.GetDom()
					query.Find(".box-series ul li ").Each(func(i int, s *goquery.Selection) {
						a := s.Find("ul.info-list .title > a")
						url, ok := a.Attr("href")
						if ok && url != "" {
							name := a.Text()
							ctx.SetTemp("name", name)
							ctx.AddQueue(&request.Request{
								Url:  parseUrl(url),
								Rule: "play",
								Temp: ctx.CopyTemps(),
							})
						}
					})
				},
			},
			"play": {
				ParseFunc: func(ctx *spider.Context) {
					query := ctx.GetDom()
					url, ok := query.Find(".base .base_info .desc-link").Attr("href")
					if ok && url != "" {
						ctx.SetTemp("play_url", ctx.GetUrl())
						ctx.AddQueue(&request.Request{
							Url:  parseUrl(url),
							Rule: "detail",
							Temp: ctx.GetTemps(),
						})
					}
				},
			},
			"detail": {
				ItemFields: []string{
					"name",
					"channel",
					"play_count",
					"comment_count",
					"tags",
					"release_at",
					"score",
					"vip",
					"date",
					"crawl_at",
					"detail_url",
					"play_url",
				},
				ParseFunc: func(ctx *spider.Context) {
					query := ctx.GetDom()
					var (
						vip, playCnt, commentCnt int
						score                    float64
						releaseAt                string
						tags                     []string
					)
					vipClass, ok := query.Find(".p-thumb .p-thumb-tagrt span").
						Attr("class")
					if ok {
						if vipClass == "vip-free" {
							vip = 1
						} else if vipClass == "vip-ticket" {
							vip = 2
						}
					}
					query.Find(".p-base > ul > li").Each(
						func(n int, s *goquery.Selection) {
							if s.HasClass("p-score") {
								score, _ = strconv.ParseFloat(s.Find(".star-num").Text(), 64)
								return
							}

							text := s.Text()
							if strings.HasPrefix(text, "类型：") {
								s.Find("a").Each(
									func(n int, p *goquery.Selection) {
										tags = append(tags, p.Text())
									},
								)
							} else if strings.HasPrefix(text, "优酷开播：") {
								r := []rune(text)
								releaseAt = strings.Replace(string(r[5:]), ",", "", -1)
							} else if strings.HasPrefix(text, "总播放数：") {
								r := []rune(text)
								playCnt, _ = strconv.Atoi(strings.Replace(string(r[5:]), ",", "", -1))
							} else if strings.HasPrefix(text, "评论：") {
								r := []rune(text)
								commentCnt, _ = strconv.Atoi(strings.Replace(string(r[4:]), ",", "", -1))
							}
						},
					)

					ctx.Output(map[string]interface{}{
						"name":          ctx.GetTemp("name", ""),
						"channel":       ctx.GetTemp("channel", 0),
						"play_count":    playCnt,
						"comment_count": commentCnt,
						"tags":          strings.Join(tags, ","),
						"release_at":    releaseAt,
						"score":         score,
						"vip":           vip,
						"date":          StartDate,
						"crawl_at":      time.Now().Unix(),
						"detail_url":    ctx.GetUrl(),
						"play_url":      ctx.GetTemp("play_url", ""),
					})
				},
			},
		},
	},
}

func parseUrl(url string) string {
	if strings.HasPrefix(url, "//") {
		return "http:" + url
	}
	return url
}
