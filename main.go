package main

import (
	"fmt"
	"log"
	"strings"
	"net/url"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/gocolly/colly" // Colly パッケージをインポート
)



func main() {
	token := os.Getenv("DISCORD_TOKEN")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session,", err)
	}

	// ハンドラー追加
	dg.AddHandler(interactionCreate)

	// Discord WebSocket接続
	err = dg.Open()
	if err != nil {
		log.Fatal("Error opening connection,", err)
	}

	// スラッシュコマンド登録
	command := &discordgo.ApplicationCommand{
		Name:        "gameinfo",
		Description: "指定したゲームの情報を取得します。",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "title",
				Description: "ゲームのタイトル",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
		},
	}
	_, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", command)
	if err != nil {
		log.Fatal("Error registering command:", err)
	}

	fmt.Println("Bot is running. Press CTRL+C to exit.")
	select {}
}

// インタラクションイベントのハンドラー
func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != "gameinfo" {
		return
	}

	// 入力タイトルを取得
	title := i.ApplicationCommandData().Options[0].StringValue()

	// ゲーム情報をスクレイピング
	games, err := scrapeErogame(title)
	if err != nil || len(games) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "ゲーム情報が見つかりませんでした。",
			},
		})
		return
	}

	// 結果を整形して返信
	var response strings.Builder
	response.WriteString("以下のゲームが見つかりました:\n")
	for _, game := range games {
		if game["title"] == "" || game["brand"] == "" || game["average"] == "" || game["release_date"] == "" {
			continue // 空のデータがあればスキップ
		}
		response.WriteString(fmt.Sprintf(
			"タイトル: %s%s\nブランド: %s\n平均点数: %s\n発売日: %s\nシナリオ: %s\n原画: %s\n声優: %s\n画像: %s\n %s\n",
			game["title"],game["platform"], game["brand"], game["average"], game["release_date"], game["shinario"], game["genga"], game["voice_actors"], game["main_image"], game["dlsite_image"],
		))
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response.String(),
		},
	})
}

func scrapeErogame(title string) ([]map[string]string, error) {
	searchURL := fmt.Sprintf("https://erogamescape.dyndns.org/~ap2/ero/toukei_kaiseki/kensaku.php?category=game&word_category=name&word=%s", url.QueryEscape(title))
	c := colly.NewCollector()
	log.Println("訪問するURL:", searchURL)
	var results []map[string]string

	// 検索結果のページを解析
	c.OnHTML("table tbody tr td a", func(e *colly.HTMLElement) {
		gameTitle := e.Text
		platform := e.DOM.Parent().Find("span[style='font-weight:bold;']").Text()
		if platform == "" {
			platform = ""
		}

		// 完全一致でタイトルを確認
		if strings.Contains(gameTitle, title) || strings.Contains(title, gameTitle) {
			// ゲームページのリンクを取得
			gameLink := "https://erogamescape.dyndns.org/~ap2/ero/toukei_kaiseki/" + e.Attr("href")

			// ゲームデータをmapに格納
			game := map[string]string{
				"title":    gameTitle,  // ゲームのタイトル
				"link":     gameLink,   // ゲームページのリンク
				"platform": platform,   // ゲームプラットフォーム
			}

			// ゲーム画像のmapを格納
			gameImages := make(map[string]string)

			// ゲームページにアクセス
			c.OnHTML("tr#brand td a", func(e *colly.HTMLElement) {
				game["brand"] = e.Text
			})

			c.OnHTML("tr#average td", func(e *colly.HTMLElement) {
				game["average"] = e.Text
			})

			c.OnHTML("tr#sellday td", func(e *colly.HTMLElement) {
				game["release_date"] = e.Text
			})

			c.OnHTML("tr#seiyu td", func(e *colly.HTMLElement) {
				game["voice_actors"] = e.Text
			})

			c.OnHTML("tr#genga td", func(e *colly.HTMLElement) {
				game["genga"] = e.Text
			})

			c.OnHTML("tr#shinario td", func(e *colly.HTMLElement) {
				game["shinario"] = e.Text
			})

			// 画像情報を取得
			c.OnHTML("div#main_image a img", func(e *colly.HTMLElement) {
				gameImages["main_image"] = e.Attr("src")
			})

			c.OnHTML("div#dlsite_sample_cg_1_main a img", func(e *colly.HTMLElement) {
				gameImages["dlsite_image"] = e.Attr("src")
			})

			// ゲームページにアクセスして詳細情報を取得
			err := c.Visit(gameLink)
			if err != nil {
				log.Println("Error visiting game page:", err)
			}

			// ゲーム画像を格納
			game["main_image"] = gameImages["main_image"]
			game["dlsite_image"] = gameImages["dlsite_image"]

			// 結果をリストに追加
			results = append(results, game)
		}
	})

	// エラー処理
	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Error during scraping:", err)
	})

	// スクレイピング実行
	err := c.Visit(searchURL)
	if err != nil {
		return nil, err
	}

	// 結果を返す
	return results, nil
}
