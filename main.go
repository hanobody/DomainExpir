package main

import (
	cfapi "DomainC/CFapi"
	"DomainC/tools"
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	alertDays        = 10
	telegramBotToken = "7800308079:AAFNidRY84WgCYBef9NRawpIZD0-5VDDB8k"
	telegramChatID   = -5246306865
)

func AlertDaysDuration() time.Duration {
	return time.Hour * 24 * time.Duration(alertDays)
}

var Bot *tgbotapi.BotAPI

func InitTelegramBot() {
	token := telegramBotToken
	if token == "" {
		log.Fatal("请先设置 TELEGRAM_BOT_TOKEN")
	}

	var err error
	Bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("创建 Bot 失败: %v", err)
	}

	Bot.Debug = false
	log.Printf("已登录 Telegram 账号: %s", Bot.Self.UserName)
}
func StartTelegramListener() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := Bot.GetUpdatesChan(u)

	for update := range updates {
		// 处理普通消息
		// if update.Message != nil {
		// 	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		// 	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "收到你的消息啦 👌")
		// 	Bot.Send(msg)
		// }

		// 处理按钮点击
		if update.CallbackQuery != nil {
			data := update.CallbackQuery.Data
			user := update.CallbackQuery.From
			log.Printf("按钮点击: %s", data)

			// 调用你写的处理逻辑
			HandleCallback(data, user)

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "操作已收到")
			Bot.Send(callback)
		}
	}
}

type Button struct {
	Text         string
	CallbackData string
}

// 发送普通提醒
func SendTelegramAlert(msg string) {
	chatID := int64(telegramChatID)
	message := tgbotapi.NewMessage(chatID, msg)
	message.ParseMode = "Markdown"
	Bot.Send(message)
}

// 发送带按钮的提醒
func SendTelegramAlertWithButtons(msg string, buttons [][]Button) {
	chatID := int64(telegramChatID)
	message := tgbotapi.NewMessage(chatID, msg)

	// 转换为 Telegram 的 InlineKeyboardMarkup
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, row := range buttons {
		var btnRow []tgbotapi.InlineKeyboardButton
		for _, b := range row {
			btnRow = append(btnRow, tgbotapi.NewInlineKeyboardButtonData(b.Text, b.CallbackData))
		}
		rows = append(rows, btnRow)
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	message.ReplyMarkup = keyboard

	Bot.Send(message)
}

// checkDomains 只执行一次逻辑
func checkDomains() {
	accounts := []cfapi.CFAccount{
		{Email: "Shengshi6688@gmail.com", APIKey: "", Label: "Shengshi6688"},
		{Email: "yuang6496@gmail.com", APIKey: "", Label: "yuang6496"},
	}

	files := []string{"aws.txt"}
	allDomains := cfapi.GetAllDomains(accounts, files)

	var expiring []cfapi.DomainSource
	for _, ds := range allDomains {
		time.Sleep(1 * time.Second) // 避免请求过快

		result := tools.CheckWhois(ds.Domain)
		expiry := tools.ExtractExpiry(result)
		fmt.Printf("%s\t|\t%s\t|\t%s\n", ds.Domain, ds.Source, expiry)

		expiryTime, err := time.Parse("2006-01-02", expiry)
		if err != nil {
			log.Printf("【WHOIS异常】 %s", result)
			continue
		}

		if time.Until(expiryTime) <= AlertDaysDuration() {
			ds.Expiry = expiry
			expiring = append(expiring, ds)
		}
	}

	if err := cfapi.SaveExpiringDomainsToFile(expiring, "expiring_domains.txt"); err != nil {
		log.Printf("保存即将到期域名失败: %v", err)
	}
}

// scheduleCheckDomains 每天15 点执行一次 checkDomains
func scheduleCheckDomains() {
	go func() {
		for {
			now := time.Now()
			// 下次15 点
			next := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location())
			if now.After(next) {
				// 已经过了今天15点，推到明天
				next = next.Add(24 * time.Hour)
			}
			duration := next.Sub(now)
			log.Printf("距离下次 checkDomains 执行还有: %v", duration)

			time.Sleep(duration) // 等到15 点
			SendTelegramAlert("每天15点开始检查域名到期，只对CF里状态为活动的域名进行到期检查")
			checkDomains() // 执行一次
		}
	}()
}

// 清空文件内容
func clearFile(filePath string) {
	file, err := os.OpenFile(filePath, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("清空文件 %s 失败: %v", filePath, err)
		return
	}
	defer file.Close()

	fmt.Printf("报警完成，已清空文件 %s，等待下次生成...\n", filePath)
}

func alertExpiringDomains(filePath string) {
	for {
		// 如果文件不存在，说明还没生成，等一会儿
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			time.Sleep(30 * time.Second)
			continue
		}

		file, err := os.Open(filePath)
		if err != nil {
			log.Printf("无法打开文件 %s: %v\n", filePath, err)
			time.Sleep(30 * time.Second)
			continue
		}

		fmt.Println("开始读取即将到期域名...")
		scanner := bufio.NewScanner(file)
		var lines []cfapi.DomainSource
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) >= 3 {
				lines = append(lines, cfapi.DomainSource{
					Domain: strings.TrimSpace(parts[0]),
					Source: strings.TrimSpace(parts[1]),
					Expiry: strings.TrimSpace(parts[2]),
				})
			}
		}
		file.Close()

		if err := scanner.Err(); err != nil {
			log.Printf("读取文件 %s 出错: %v", filePath, err)
			time.Sleep(30 * time.Second)
			continue
		}

		// ---- 报警逻辑 ----
		for _, ds := range lines {

			days, err := tools.DaysUntilExpiry(ds.Expiry)
			if err != nil {
				log.Printf("无法计算剩余天数: %v", err)
				continue
			}

			if ds.Source == "Shengshi6688" || ds.Source == "yuang6496" {
				msg := fmt.Sprintf("【域名即将到期】\n域名: %s\n来源: %s\n到期时间: %s\n 注意：如果没人响应，遇到到期后将自动从CF删除",
					ds.Domain, ds.Source, ds.Expiry)
				accountLabel := ds.Source
				fmt.Println("发送 Telegram 消息:", msg)
				time.Sleep(3 * time.Second) // 避免消息过快
				SendTelegramAlertWithButtons(msg, [][]Button{
					{
						{Text: "暂停域名", CallbackData: fmt.Sprintf("pause|%s|%s|%s", accountLabel, ds.Domain, "yes")},
						{Text: "恢复暂停", CallbackData: fmt.Sprintf("pause|%s|%s|%s", accountLabel, ds.Domain, "no")},
						{Text: "查询解析", CallbackData: fmt.Sprintf("DNS|%s|%s", accountLabel, ds.Domain)},
					},
				})
				if days == 1 {
					account := cfapi.GetAccountByLabel(accountLabel)
					if account == nil {
						log.Printf("未找到账号: %s", accountLabel)
						continue
					}
					go func(acc cfapi.CFAccount, domain string) {
						err := cfapi.DeleteCloudflareDomain(acc, domain)
						if err != nil {
							SendTelegramAlert(fmt.Sprintf("⚠️ 自动删除域名失败: %s (%v)", domain, err))
						} else {
							SendTelegramAlert(fmt.Sprintf("✅ 已自动删除即将到期的域名: %s", domain))
						}
					}(*account, ds.Domain)
				}
			} else {
				msg := fmt.Sprintf("【域名即将到期】\n域名: %s\n来源: %s\n到期时间: %s ",
					ds.Domain, ds.Source, ds.Expiry)
				SendTelegramAlert(msg + "\n非CF账户的域名请手工处理。")
			}
		}

		// 调用清空文件函数
		clearFile(filePath)

		// 休息一会再检查文件是否重新生成
		time.Sleep(1 * time.Minute)
	}
}

func HandleCallback(callbackData string, user *tgbotapi.User) {
	parts := strings.Split(callbackData, "|")
	if len(parts) < 3 {
		log.Printf("无效的回调数据: %s", callbackData)
		return
	}

	action := parts[0]
	accountLabel := parts[1]
	domain := parts[2]

	paused := ""
	if len(parts) >= 4 {
		paused = parts[3]
	}
	fmt.Println("处理回调数据:", action, accountLabel, domain)
	switch action {
	case "pause":
		go func() { // 异步执行暂停
			account := cfapi.GetAccountByLabel(accountLabel)
			if account == nil {
				log.Printf("未找到账号: %s", accountLabel)
				return
			}
			var successMsg, failMsg string
			if paused == "yes" {
				successMsg = fmt.Sprintf("%s禁用域名成功: %s---%s", user.UserName, domain, accountLabel)
				failMsg = fmt.Sprintf("%s禁用域名失败: %s-----%s (%v)", user.UserName, domain, accountLabel, "%v")
			} else {
				successMsg = fmt.Sprintf("%s解除禁用成功: %s---%s", user.UserName, domain, accountLabel)
				failMsg = fmt.Sprintf("%s解除禁用失败: %s-----%s (%v)", user.UserName, domain, accountLabel, "%v")
			}
			err := cfapi.PauseCloudflareDomain(*account, domain, paused)
			if err != nil {
				SendTelegramAlert(fmt.Sprintf(failMsg, err))
			} else {
				SendTelegramAlert(successMsg)
			}
		}()
	case "DNS":
		go func() { // 异步执行暂停
			account := cfapi.GetAccountByLabel(accountLabel)
			if account == nil {
				log.Printf("未找到账号: %s", accountLabel)
				return
			}
			records, err := cfapi.ListCloudflareDNSRecords(*account, domain)
			if err != nil {
				SendTelegramAlert(fmt.Sprintf("查询域名解析失败: %s-----%s (%v)", domain, accountLabel, err))
			}
			if len(records) == 0 {
				SendTelegramAlert(fmt.Sprintf("域名 %s -----%s 没有任何解析记录。", domain, accountLabel))
				return
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("【域名解析记录】\n域名: %s\n来源: %s\n\n", domain, accountLabel))

			for _, r := range records {
				sb.WriteString(fmt.Sprintf(
					"%s  %s → %s (%v)\n",
					r.Type, r.Name, r.Content, *r.Proxied,
				))
			}

			// 发送到 Telegram
			SendTelegramAlert(sb.String())

		}()
	}
}

func main() {
	InitTelegramBot()

	go StartTelegramListener()
	scheduleCheckDomains()                          //每天定时后台执行，执行的时候不影响其他进程
	go alertExpiringDomains("expiring_domains.txt") //监听文件变动，如果有新内容则发送提醒

	select {}
}
