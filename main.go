package main

import (
	"DomainC/callback"
	"DomainC/cfclient"
	"DomainC/config"
	"DomainC/domain"
	"DomainC/scheduler"
	"DomainC/telegram"
	"DomainC/tools"
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func AlertDaysDuration() time.Duration {
	return time.Hour * 24 * time.Duration(config.Cfg.AlertDays)
}

func GetAllDomains(accounts []config.CF, filePaths []string) []domain.DomainSource {
	svc := domain.NewService(cfclient.NewClient())
	ds, err := svc.CollectAll(accounts, filePaths)
	if err != nil {
		log.Printf("收集域名时发生错误: %v", err)
	}
	return ds
}
func SaveExpiringDomainsToFile(domains []domain.DomainSource, filePath string) error {

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, ds := range domains {
		line := fmt.Sprintf(
			"%s|%s|%s\n",
			ds.Domain,
			ds.Source,
			ds.Expiry,
		)
		if _, err := writer.WriteString(line); err != nil {
			return fmt.Errorf("写入文件失败: %v", err)
		}
	}

	return writer.Flush()
}

// checkDomains 只执行一次逻辑
func checkDomains() {
	files := config.Cfg.DomainFiles
	allDomains := GetAllDomains(config.Cfg.CloudflareAccounts, files)

	var expiring []domain.DomainSource
	for _, ds := range allDomains {
		time.Sleep(1 * time.Second) // 避免请求过快

		result := tools.CheckWhois(ds.Domain)
		expiry := tools.ExtractExpiry(result)

		fmt.Printf(
			"%s\t|\t%s\t|\t%s\n",
			ds.Domain,
			ds.Source,
			expiry,
		)

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

	if err := SaveExpiringDomainsToFile(
		expiring,
		"expiring_domains.txt",
	); err != nil {
		log.Printf("保存即将到期域名失败: %v", err)
	}
}

// 清空文件内容
func clearFile(filePath string) {
	file, err := os.OpenFile(
		filePath,
		os.O_TRUNC|os.O_WRONLY,
		0644,
	)
	if err != nil {
		log.Printf(
			"清空文件 %s 失败: %v",
			filePath,
			err,
		)
		return
	}
	defer file.Close()

	fmt.Printf(
		"报警完成，已清空文件 %s，等待下次生成...\n",
		filePath,
	)
}

func alertExpiringDomains(filePath string) {
	for {
		// 如果文件不存在，说明还没生成
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			time.Sleep(30 * time.Second)
			continue
		}

		file, err := os.Open(filePath)
		if err != nil {
			log.Printf(
				"无法打开文件 %s: %v\n",
				filePath,
				err,
			)
			time.Sleep(30 * time.Second)
			continue
		}

		fmt.Println("开始读取即将到期域名...")
		scanner := bufio.NewScanner(file)

		var lines []domain.DomainSource
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			parts := strings.Split(line, "|")
			if len(parts) >= 3 {
				lines = append(lines, domain.DomainSource{
					Domain: strings.TrimSpace(parts[0]),
					Source: strings.TrimSpace(parts[1]),
					Expiry: strings.TrimSpace(parts[2]),
				})
			}
		}
		file.Close()

		if err := scanner.Err(); err != nil {
			log.Printf(
				"读取文件 %s 出错: %v",
				filePath,
				err,
			)
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

				msg := fmt.Sprintf(
					"【域名即将到期】\n域名: %s\n来源: %s\n到期时间: %s\n注意：如果没人响应，遇到到期后将自动从CF删除",
					ds.Domain,
					ds.Source,
					ds.Expiry,
				)

				accountLabel := ds.Source
				fmt.Println("发送 Telegram 消息:", msg)

				time.Sleep(3 * time.Second)

				telegram.SendTelegramAlertWithButtons(
					msg,
					[][]telegram.Button{
						{
							{
								Text: "暂停域名",
								CallbackData: fmt.Sprintf(
									"pause|%s|%s|%s",
									accountLabel,
									ds.Domain,
									"yes",
								),
							},
							{
								Text: "恢复暂停",
								CallbackData: fmt.Sprintf(
									"pause|%s|%s|%s",
									accountLabel,
									ds.Domain,
									"no",
								),
							},
							{
								Text: "查询解析",
								CallbackData: fmt.Sprintf(
									"DNS|%s|%s",
									accountLabel,
									ds.Domain,
								),
							},
						},
					},
				)

				if days == 1 {
					account := cfclient.GetAccountByLabel(accountLabel)
					if account == nil {
						log.Printf(
							"未找到账号: %s",
							accountLabel,
						)
						continue
					}

					go func(acc config.CF, domain string) {
						err := cfclient.DeleteDomain(acc, domain)
						if err != nil {
							telegram.SendTelegramAlert(
								fmt.Sprintf(
									"⚠️ 自动删除域名失败: %s (%v)",
									domain,
									err,
								),
							)
						} else {
							telegram.SendTelegramAlert(
								fmt.Sprintf(
									"✅ 已自动删除即将到期的域名: %s",
									domain,
								),
							)
						}
					}(*account, ds.Domain)
				}

			} else {
				msg := fmt.Sprintf(
					"【域名即将到期】\n域名: %s\n来源: %s\n到期时间: %s",
					ds.Domain,
					ds.Source,
					ds.Expiry,
				)
				telegram.SendTelegramAlert(
					msg + "\n非CF账户的域名请手工处理。",
				)
			}
		}

		// 调用清空文件函数
		clearFile(filePath)

		// 休息一会再检查文件是否重新生成
		time.Sleep(1 * time.Minute)
	}
}

func main() {
	config.Load("config.yaml")
	telegram.Init()
	go telegram.StartListener(callback.HandleCallback)

	checkDomains()
	scheduler.ScheduleDailyAt(15, 0, func() {
		telegram.SendTelegramAlert("每天15点开始检查域名到期，只对CF里状态为活动的域名进行到期检查")
		checkDomains()
	})
	go alertExpiringDomains("expiring_domains.txt")

	select {}
}
