package callback

import (
	"DomainC/cfclient"
	"DomainC/telegram"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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
		go func() {
			account := cfclient.GetAccountByLabel(accountLabel)
			if account == nil {
				log.Printf("未找到账号: %s", accountLabel)
				return
			}

			var successMsg, failMsg string
			if paused == "yes" {
				successMsg = fmt.Sprintf("%s禁用域名成功: %s---%s", user.UserName, domain, accountLabel)
				failMsg = fmt.Sprintf("%s禁用域名失败: %s-----%s (%%v)", user.UserName, domain, accountLabel)
			} else {
				successMsg = fmt.Sprintf("%s解除禁用成功: %s---%s", user.UserName, domain, accountLabel)
				failMsg = fmt.Sprintf("%s解除禁用失败: %s-----%s (%%v)", user.UserName, domain, accountLabel)
			}

			err := cfclient.PauseDomain(*account, domain, paused == "yes")
			if err != nil {
				telegram.SendTelegramAlert(fmt.Sprintf(failMsg, err))
			} else {
				telegram.SendTelegramAlert(successMsg)
			}
		}()

	case "DNS":
		go func() {
			account := cfclient.GetAccountByLabel(accountLabel)
			if account == nil {
				log.Printf("未找到账号: %s", accountLabel)
				return
			}

			records, err := cfclient.ListDNSRecords(*account, domain)
			if err != nil {
				telegram.SendTelegramAlert(fmt.Sprintf("查询域名解析失败: %s-----%s (%v)", domain, accountLabel, err))
				return
			}

			if len(records) == 0 {
				telegram.SendTelegramAlert(fmt.Sprintf("域名 %s -----%s 没有任何解析记录。", domain, accountLabel))
				return
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("【域名解析记录】\n域名: %s\n来源: %s\n\n", domain, accountLabel))

			for _, r := range records {
				sb.WriteString(fmt.Sprintf("%s %s → %s (%v)\n", r.Type, r.Name, r.Content, *r.Proxied))
			}

			telegram.SendTelegramAlert(sb.String())
		}()
	// case "delete":
	// 	go func() {
	// 		account := cfclient.GetAccountByLabel(accountLabel)
	// 		if account == nil {
	// 			log.Printf("未找到账号: %s", accountLabel)
	// 			return
	// 		}
	// 		err := cfclient.DeleteDomain(*account, domain)
	// 		if err != nil {
	// 			telegram.SendTelegramAlert(fmt.Sprintf("删除域名失败: %s-----%s (%v)", domain, accountLabel, err))
	// 			return
	// 		}
	// 		telegram.SendTelegramAlert(fmt.Sprintf("删除域名成功: %s-----%s", domain, accountLabel))
	// 	}()
	case "delete":
		go func() {
			confirmMsg := fmt.Sprintf(
				"⚠️【删除二次确认】\n操作人: %s\n域名: %s\n账号: %s\n\n此操作不可逆，确认要删除该域名（Cloudflare Zone）吗？",
				user.UserName, domain, accountLabel,
			)

			buttons := [][]telegram.Button{{
				{Text: "✅ 确认删除", CallbackData: fmt.Sprintf("delete_confirm|%s|%s", accountLabel, domain)},
				{Text: "❌ 取消", CallbackData: fmt.Sprintf("delete_cancel|%s|%s", accountLabel, domain)},
			}}

			telegram.SendTelegramAlertWithButtons(confirmMsg, buttons)
		}()

	case "delete_confirm":
		go func() {
			account := cfclient.GetAccountByLabel(accountLabel)
			if account == nil {
				log.Printf("未找到账号: %s", accountLabel)
				return
			}

			err := cfclient.DeleteDomain(*account, domain)
			// err := NewClient.DeleteDomain(context.Background(), account, domain)
			if err != nil {
				telegram.SendTelegramAlert(fmt.Sprintf("删除域名失败: %s-----%s (%v)", domain, accountLabel, err))
				return
			}
			telegram.SendTelegramAlert(fmt.Sprintf("✅ 删除域名成功: %s-----%s (操作人:%s)", domain, accountLabel, user.UserName))
		}()

	case "delete_cancel":
		go func() {
			telegram.SendTelegramAlert(fmt.Sprintf("已取消删除: %s-----%s (操作人:%s)", domain, accountLabel, user.UserName))
		}()
	}

}
