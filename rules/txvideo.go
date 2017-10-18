package rules

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/henrylee2cn/pholcus/app/downloader/request"
	"github.com/henrylee2cn/pholcus/app/spider"
	"github.com/henrylee2cn/pholcus/common/goquery"
	"github.com/henrylee2cn/pholcus/logs"
)

const (
	Thousand = 1000
	Million  = 100000000
)

var TXVideoChannels = []string{
	"http://v.qq.com/x/list/movie",
	"http://v.qq.com/x/list/tv",
	"http://v.qq.com/x/list/variety",
	"http://v.qq.com/x/list/cartoon",
	"http://v.qq.com/x/list/children",
}

func init() {
	TXVideo.Register()
}

func TXVideoNamespace(*spider.Spider) string {
	return "tx"
}

func TXVideoSubNamespace(*spider.Spider, map[string]interface{}) string {
	return "videos"
}

var TXVideo = &spider.Spider{
	Name:            "腾讯视频详情",
	Description:     "腾讯视频详情[v.qq.com/x/list]",
	Namespace:       TXVideoNamespace,
	SubNamespace:    TXVideoSubNamespace,
	NotDefaultField: true,
	// Pausetime:    300,
	// Keyin:        KEYIN,
	EnableCookie: false,
	RuleTree: &spider.RuleTree{
		Root: func(ctx *spider.Context) {
			for index, url := range TXVideoChannels {
				ctx.AddQueue(&request.Request{
					Url:  url,
					Rule: "pages",
					Temp: map[string]interface{}{
						"baseUrl": url,
						"channel": index,
					},
				})
				time.Sleep(time.Minute * 5)
				// ctx.SetTimer(strconv.Itoa(index), time.Minute*3, nil)
			}
		},
		Trunk: map[string]*spider.Rule{
			"pages": {
				AidFunc: func(ctx *spider.Context, aid map[string]interface{}) interface{} {
					baseUrl := ctx.GetTemp("baseUrl", "").(string)
					if !strings.Contains(baseUrl, "?") {
						baseUrl += "?"
					}
					channel := ctx.GetTemp("channel", 0).(int)
					for loop := aid["loop"].([2]int); loop[0] < loop[1]; loop[0]++ {
						ctx.AddQueue(&request.Request{
							Url:  fmt.Sprintf("%s&offset=%d", baseUrl, 30*loop[0]),
							Rule: aid["Rule"].(string),
							Temp: map[string]interface{}{
								"channel": channel,
							},
						})
						time.Sleep(time.Second * 20)
						// ctx.SetTimer(fmt.Sprintf("%d_%d", channel, loop[0]), time.Minute*3, nil)
					}
					return nil
				},
				ParseFunc: func(ctx *spider.Context) {
					query := ctx.GetDom()
					totalText := query.Find(".mod_pages ._items a").Last().Text()
					total, err := strconv.Atoi(totalText)
					if total == 0 || err != nil {
						logs.Log.Critical("[消息提示：| 任务：%v | KEYIN：%v | 规则：%v] 没有抓取到任何数据！!!\n", ctx.GetName(), ctx.GetKeyin(), ctx.GetRuleName())
						return
					}
					logs.Log.Informational("channel: %s, totalPages: %d", TXVideoChannels[ctx.GetTemp("channel", 0).(int)], total)
					ctx.Aid(map[string]interface{}{
						"loop": [2]int{1, total},
						"Rule": "list",
					})
					ctx.Parse("list")
				},
			},
			"list": {
				ParseFunc: func(ctx *spider.Context) {
					query := ctx.GetDom()
					query.Find(".figures_list li>a.figure").Each(func(i int, s *goquery.Selection) {
						if url, ok := s.Attr("href"); ok {
							var vip int
							vipText, ok := s.Find(".mark_v img").Attr("alt")
							if ok {
								if vipText == "VIP" {
									vip = 1
								} else if vipText == "付费" {
									vip = 2
								}
							}
							ctx.SetTemp("vip", vip)
							ctx.AddQueue(&request.Request{
								Url:  url,
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
					url, ok := query.Find(".mod_player_side .player_title a").Attr("href")
					if ok {
						var playCount float64
						playCountText := query.Find(".mod_action .action_title .icon_text em").Text()
						if playCountText != "" {
							r := []rune(playCountText)
							unit := string(r[len(r)-1:])
							playCount, _ = strconv.ParseFloat(string(r[:len(r)-1]), 64)
							if unit == "亿" {
								playCount *= Million
							} else if unit == "万" {
								playCount *= 10 * Thousand
							}
						}
						ctx.SetTemp("play_count", int64(playCount))
						ctx.SetTemp("play_url", cutURL(ctx.GetUrl()))
						ctx.AddQueue(&request.Request{
							Url:  "https://" + ctx.GetHost() + url,
							Rule: "detail",
							Temp: ctx.CopyTemps(),
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
					name := query.Find(".video_title_cn a").Text()

					var releaseAt string
					query.Find(".video_type .type_item").EachWithBreak(func(n int, s *goquery.Selection) bool {
						title := s.Children().First().Text()
						if title == "出品时间:" || title == "上映时间:" {
							releaseAt = s.Children().Last().Text()
							return false
						}
						return true
					})
					scoreText := query.Find(".video_score .score_v .score").Text()
					score, _ := strconv.ParseFloat(scoreText, 64)
					var tags []string
					query.Find(".video_tag .tag_list a.tag").Each(func(n int, s *goquery.Selection) {
						tags = append(tags, s.Text())
					})

					ctx.Output(map[string]interface{}{
						"name":          name,
						"channel":       ctx.GetTemp("channel", 0),
						"play_count":    ctx.GetTemp("play_count", ""),
						"comment_count": 0,
						"tags":          strings.Join(tags, ","),
						"release_at":    releaseAt,
						"score":         score,
						"vip":           ctx.GetTemp("vip", 0),
						"date":          StartDate,
						"crawl_at":      time.Now().Unix(),
						"detail_url":    cutURL(ctx.GetUrl()),
						"play_url":      ctx.GetTemp("play_url", ""),
					})
				},
			},
		},
	},
}
