package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var massiveProxyPool = []string{
	"103.152.112.5:80",
	"185.199.229.156:8080",
	"45.33.32.156:3128",
	"190.61.35.153:8080",
	"200.105.215.12:80",
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

	cmdClone := "cloner"
	commandClone := &discordgo.ApplicationCommand{
		Name:        cmdClone,
		Description: "نسخ سيرفر كامل باستخدام التوكن الشخصي للمصدر وبوتات التنفيذ",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "template_guild_id",
				Description: "إيدي السيرفر المراد نسخه",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "user_token",
				Description: "التوكن الشخصي لحسابك (لسحب السيرفر المصدر)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "bot_tokens",
				Description: "توكنات البوتات لتنفيذ الإنشاء (مفصولة بفاصلة)",
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
					Content: "❌ للمشرفين فقط.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		data := i.ApplicationCommandData()

		if data.Name == cmdClone {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🔄 جاري بدء عملية نسخ السيرفر بالتوكن الشخصي...",
				},
			})

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range data.Options {
				optionMap[opt.Name] = opt
			}

			templateGuildID := optionMap["template_guild_id"].StringValue()
			userToken := optionMap["user_token"].StringValue()
			tokensInput := optionMap["bot_tokens"].StringValue()

			var tokens []string
			for _, t := range strings.Split(tokensInput, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tokens = append(tokens, t)
				}
			}

			go executeServerClonerWithUserToken(tokens, userToken, massiveProxyPool, i.GuildID, templateGuildID, i.Interaction, s)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في تشغيل الجلسة:", err)
		return
	}

	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandClone)
	fmt.Println("⚡ أمر النسخ بالتوكن الشخصي جاهز!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func filterActiveProxies(rawProxies []string) []*http.Client {
	var activeClients []*http.Client
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, pStr := range rawProxies {
		wg.Add(1)
		go func(proxy string) {
			defer wg.Done()
			pURL, err := url.Parse("http://" + proxy)
			if err != nil {
				return
			}
			client := &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(pURL),
				},
				Timeout: 1500 * time.Millisecond,
			}
			req, _ := http.NewRequest("GET", "https://discord.com/api/v10/gateway", nil)
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				mu.Lock()
				activeClients = append(activeClients, client)
				mu.Unlock()
			}
		}(pStr)
	}
	wg.Wait()
	return activeClients
}

func updateInteractionStatus(s *discordgo.Session, interaction *discordgo.Interaction, content string) {
	_, _ = s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}

// دالة لجلب بيانات السيرفر المصدر باستخدام التوكن الشخصي مباشرة عبر HTTP
func fetchGuildDataWithUserToken(userToken, guildID string) ([]map[string]interface{}, []map[string]interface{}, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// جلب الرومات
	reqChan, _ := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/channels", guildID), nil)
	reqChan.Header.Set("Authorization", userToken) // التوكن الشخصي يرسل مباشرة بدون كلمة Bot
	reqChan.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	respChan, err := client.Do(reqChan)
	if err != nil {
		return nil, nil, err
	}
	defer respChan.Body.Close()

	var channels []map[string]interface{}
	json.NewDecoder(respChan.Body).Decode(&channels)

	// جلب الرتب
	reqRole, _ := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/roles", guildID), nil)
	reqRole.Header.Set("Authorization", userToken)
	reqRole.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	respRole, err := client.Do(reqRole)
	if err != nil {
		return channels, nil, err
	}
	defer respRole.Body.Close()

	var roles []map[string]interface{}
	json.NewDecoder(respRole.Body).Decode(&roles)

	return channels, roles, nil
}

func executeServerClonerWithUserToken(tokens []string, userToken string, rawProxies []string, guildID, templateGuildID string, interaction *discordgo.Interaction, s *discordgo.Session) {
	if len(tokens) == 0 {
		updateInteractionStatus(s, interaction, "❌ خطأ: لم يتم إدخال توكنات بوتات التنفيذ.")
		return
	}

	clients := filterActiveProxies(rawProxies)
	if len(clients) == 0 {
		clients = append(clients, &http.Client{Timeout: 2 * time.Second})
	}

	makeProxyRequest := func(method, urlStr string, body []byte, index int) (int, bool) {
		client := clients[index%len(clients)]
		token := tokens[index%len(tokens)]

		for attempt := 0; attempt < 3; attempt++ {
			var bodyBuf *bytes.Buffer
			if body != nil {
				bodyBuf = bytes.NewBuffer(body)
			} else {
				bodyBuf = bytes.NewBuffer(nil)
			}

			req, err := http.NewRequestWithContext(context.Background(), method, urlStr, bodyBuf)
			if err != nil {
				return 0, false
			}
			req.Header.Set("Authorization", "Bot "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

			resp, err := client.Do(req)
			if err != nil {
				client = clients[(index+attempt+1)%len(clients)]
				continue
			}

			statusCode := resp.StatusCode
			resp.Body.Close()

			if statusCode == 429 {
				time.Sleep(50 * time.Millisecond)
				continue
			}

			return statusCode, true
		}
		return 0, false
	}

	updateInteractionStatus(s, interaction, "🔍 جاري سحب بيانات السيرفر المصدر باستخدام التوكن الشخصي...")
	templateChannels, templateRoles, err := fetchGuildDataWithUserToken(userToken, templateGuildID)
	if err != nil {
		updateInteractionStatus(s, interaction, "❌ فشل سحب البيانات، تأكد من صحة التوكن الشخصي وإلزامية وجود حسابك بالمصدر.")
		return
	}

	// حذف الرومات والرتب القديمة بالسيرفر المستهدف باستخدام البوتات
	channels, err := s.GuildChannels(guildID)
	var oldChannelIDs []string
	if err == nil {
		for _, ch := range channels {
			if ch != nil {
				oldChannelIDs = append(oldChannelIDs, ch.ID)
			}
		}
	}

	roles, err := s.GuildRoles(guildID)
	if err == nil {
		updateInteractionStatus(s, interaction, "🗑️ جاري حذف الرتب القديمة...")
		var wgRole sync.WaitGroup
		for i, r := range roles {
			if !r.Managed && r.Name != "@everyone" {
				wgRole.Add(1)
				go func(rID string, idx int) {
					defer wgRole.Done()
					makeProxyRequest("DELETE", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/roles/%s", guildID, rID), nil, idx)
				}(r.ID, i)
			}
		}
		wgRole.Wait()
	}

	updateInteractionStatus(s, interaction, fmt.Sprintf("🧹 جاري حذف الرومات القديمة (%d روم)...", len(oldChannelIDs)))
	chQueue := make(chan string, len(oldChannelIDs))
	for _, id := range oldChannelIDs {
		chQueue <- id
	}
	close(chQueue)

	var wgDel sync.WaitGroup
	for w := 0; w < 60; w++ {
		wgDel.Add(1)
		go func(workerID int) {
			defer wgDel.Done()
			for chID := range chQueue {
				makeProxyRequest("DELETE", "https://discord.com/api/v10/channels/"+chID, nil, workerID)
			}
		}(w)
	}
	wgDel.Wait()

	updateInteractionStatus(s, interaction, "📥 جاري إنشاء الرتب المستنسخة...")
	var wgCreateRole sync.WaitGroup
	for i, r := range templateRoles {
		name, _ := r["name"].(string)
		if name == "@everyone" {
			continue
		}
		wgCreateRole.Add(1)
		go func(role map[string]interface{}, idx int) {
			defer wgCreateRole.Done()
			rolePayload, _ := json.Marshal(map[string]interface{}{
				"name":        role["name"],
				"permissions": role["permissions"],
				"color":       role["color"],
				"hoist":       role["hoist"],
				"mentionable": role["mentionable"],
			})
			makeProxyRequest("POST", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/roles", guildID), rolePayload, idx)
		}(r, i)
	}
	wgCreateRole.Wait()

	updateInteractionStatus(s, interaction, "📂 جاري إنشاء الفئات والرومات...")
	var categories []map[string]interface{}
	var regularChannels []map[string]interface{}

	for _, ch := range templateChannels {
		if t, ok := ch["type"].(float64); ok {
			if int(t) == 4 {
				categories = append(categories, ch)
			} else if int(t) == 0 || int(t) == 2 {
				regularChannels = append(regularChannels, ch)
			}
		}
	}

	categoryMap := make(map[string]string)
	var catMu sync.Mutex

	var wgCat sync.WaitGroup
	for i, cat := range categories {
		wgCat.Add(1)
		go func(c map[string]interface{}, idx int) {
			defer wgCat.Done()
			payload, _ := json.Marshal(map[string]interface{}{
				"name":     c["name"],
				"type":     c["type"],
				"position": c["position"],
			})

			client := clients[idx%len(clients)]
			token := tokens[idx%len(tokens)]
			req, err := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(payload))
			if err != nil {
				return
			}
			req.Header.Set("Authorization", "Bot "+token)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err == nil {
				if resp.StatusCode == 200 || resp.StatusCode == 201 {
					var res map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&res) == nil {
						if oldID, ok := c["id"].(string); ok {
							if newID, ok := res["id"].(string); ok {
								catMu.Lock()
								categoryMap[oldID] = newID
								catMu.Unlock()
							}
						}
					}
				}
				resp.Body.Close()
			}
		}(cat, i)
	}
	wgCat.Wait()

	var wgChannels sync.WaitGroup
	for i, ch := range regularChannels {
		wgChannels.Add(1)
		go func(c map[string]interface{}, idx int) {
			defer wgChannels.Done()

			data := map[string]interface{}{
				"name":                c["name"],
				"type":                c["type"],
				"topic":               c["topic"],
				"nsfw":                c["nsfw"],
				"rate_limit_per_user": c["rate_limit_per_user"],
				"position":            c["position"],
			}

			catMu.Lock()
			if parentID, ok := c["parent_id"].(string); ok && parentID != "" {
				if newParentID, exists := categoryMap[parentID]; exists {
					data["parent_id"] = newParentID
				}
			}
			catMu.Unlock()

			payload, _ := json.Marshal(data)
			makeProxyRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", payload, idx)
		}(ch, i)
	}
	wgChannels.Wait()

	updateInteractionStatus(s, interaction, "✅ تم اكتمال النسخ بالكامل باستخدام التوكن الشخصي وبوتات التنفيذ! 🚀")
}
