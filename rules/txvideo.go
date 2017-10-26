package rules

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/henrylee2cn/pholcus/app/downloader/request"
	"github.com/henrylee2cn/pholcus/app/spider"
	"github.com/henrylee2cn/pholcus/common/goquery"
	"github.com/henrylee2cn/pholcus/logs"
)

const (
	TenThousand = 10000
	Million     = 100000000
)

var (
	TXVideoChannels = []string{
		"http://v.qq.com/x/list/movie",
		"http://v.qq.com/x/list/tv",
		"http://v.qq.com/x/list/variety",
		"http://v.qq.com/x/list/cartoon",
		"http://v.qq.com/x/list/children",
	}
	TXVideoVIPActivityURL = "http://film.qq.com/vip/activity_v2.html"
)

func init() {
	TXVideo.Register()
}

func TXVideoNamespace(*spider.Spider) string {
	return "tx"
}

var TXVideo = &spider.Spider{
	Name:            "腾讯视频详情",
	Description:     "腾讯视频详情[v.qq.com/x/list]",
	Namespace:       TXVideoNamespace,
	NotDefaultField: true,
	// Pausetime:    300,
	// Keyin:        KEYIN,
	EnableCookie: false,
	RuleTree: &spider.RuleTree{
		Root: func(ctx *spider.Context) {
			// VIP activity
			ctx.AddQueue(&request.Request{
				Url:          TXVideoVIPActivityURL,
				Rule:         "vip_activities",
				Priority:     3,
				DownloaderID: 1,
			})
			for index, url := range TXVideoChannels {
				ctx.AddQueue(&request.Request{
					Url:  url,
					Rule: "pages",
					Temp: map[string]interface{}{
						"baseUrl": url,
						"channel": index,
					},
					Priority: 0,
				})
			}
		},
		Trunk: map[string]*spider.Rule{
			"vip_activities": {
				// ItemFields: []string{
				// "title",
				// "period",
				// "status",
				// "link",
				// "thumb",
				// "date",
				// },
				ParseFunc: func(ctx *spider.Context) {
					query := ctx.GetDom()
					query.Find("#vip-act > .mod_activity").Each(func(i int, s *goquery.Selection) {
						var thumb string
						thumbURL, ok := s.Find(".activity_cover a img").Attr("src")
						if ok {
							resp, err := http.Get(thumbURL)
							if err != nil {
								return
							}
							defer resp.Body.Close()
							body, err := ioutil.ReadAll(resp.Body)
							if err != nil {
								return
							}
							thumb = base64.StdEncoding.EncodeToString(body)
						}
						title := s.Find(".activity_info .title").Text()
						txt := s.Find(".activity_info .txt")
						desc := strings.TrimSpace(txt.First().Text())

						// period
						var start, end int64
						period := txt.Last().Text()
						temp := strings.Split(period, "：")
						if len(temp) == 2 {
							period = temp[1]
							temp = strings.Split(period, "-")
							if len(temp) == 2 {

							}
							startStr := temp[0]
							endStr := temp[1]
							startDate, _ := time.Parse("2006年01月02日", startStr)
							endDate, _ := time.Parse("2006年01月02日", endStr)
							start = startDate.Unix()
							end = endDate.Unix()
						}

						linkItem := s.Find(".activity_info > a.btn_check_event")
						link, _ := linkItem.Attr("href")
						isActive := 0
						if linkItem.HasClass("ended") {
							isActive = 1
						}
						ctx.Output(map[string]interface{}{
							"title":     title,
							"desc":      desc,
							"start":     start,
							"end":       end,
							"thumb":     thumb,
							"link":      link,
							"is_active": isActive,
							"date":      StartDate,
							"crawl_at":  time.Now().Unix(),
						})
					})
				},
			},
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
							Priority: 1,
						})
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
							mark, ok := s.Find(".mark_v img").Attr("alt")
							if ok {
								ctx.SetTemp("mark", mark)
							}
							ctx.AddQueue(&request.Request{
								Url:      url,
								Rule:     "play",
								Temp:     ctx.CopyTemps(),
								Priority: 2,
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
							if unit == "亿" || unit == "万" {
								playCount, _ = strconv.ParseFloat(string(r[:len(r)-1]), 64)
								if unit == "亿" {
									playCount *= Million
								} else if unit == "万" {
									playCount *= TenThousand
								}
							} else {
								playCount, _ = strconv.ParseFloat(playCountText, 64)
							}
						}
						ctx.SetTemp("play_count", int64(playCount))
						ctx.SetTemp("play_url", cutURL(ctx.GetUrl()))
						ctx.AddQueue(&request.Request{
							Url:      "https://" + ctx.GetHost() + url,
							Rule:     "videos",
							Temp:     ctx.CopyTemps(),
							Priority: 3,
						})
					}
				},
			},
			"videos": {
				ItemFields: []string{
					"name",
					"channel",
					"play_count",
					"comment_count",
					"tags",
					"release_at",
					"score",
					"mark",
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
						if title == "出品时间:" || title == "上映时间:" || title == "首播时间:" {
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
						"mark":          ctx.GetTemp("mark", 0),
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
