package telegram

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"DomainC/cfclient"
	"DomainC/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CommandHandler 处理群组中的命令消息
// 需要传入 Cloudflare 客户端与账号列表。
type CommandHandler struct {
	CFClient cfclient.Client
	Accounts []config.CF
	Sender   Sender
	ChatID   int64
}

func NewCommandHandler(cf cfclient.Client, sender Sender, accounts []config.CF, chatID int64) *CommandHandler {
	if cf == nil {
		cf = cfclient.NewClient()
	}
	if sender == nil {
		sender = DefaultSender()
	}
	return &CommandHandler{CFClient: cf, Accounts: accounts, Sender: sender, ChatID: chatID}
}

// HandleMessage 分发 Telegram 文本命令
func (h *CommandHandler) HandleMessage(msg *tgbotapi.Message) {
	if msg == nil || msg.Text == "" {
		return
	}
	if h.ChatID != 0 && msg.Chat != nil && msg.Chat.ID != h.ChatID {
		return
	}
	if !msg.IsCommand() {
		return
	}

	args := strings.Fields(msg.CommandArguments())
	switch msg.Command() {
	case "dns":
		go h.handleDNSCommand(strings.ToLower(msg.Command()), args)
	case "getns":
		go h.handleGetNSCommand(args)
	case "status":
		go h.handleStatusCommand(args)
	case "setdns":
		go h.handleSetDNSCommand(args)
	}
}

func (h *CommandHandler) handleDNSCommand(_ string, args []string) {
	if len(args) < 1 {
		h.sendText("用法: /dns <domain.com>")
		return
	}
	domain := strings.ToLower(args[0])

	account, zone, err := h.findZone(domain)
	if err != nil {
		if errors.Is(err, cfclient.ErrZoneNotFound) {
			h.sendText(fmt.Sprintf("域名 %s 不属于任何 Cloudflare 账号。", domain))
			return
		}
		h.sendText(fmt.Sprintf("查询域名失败: %v", err))
		return
	}

	records, err := h.CFClient.ListDNSRecords(context.Background(), *account, zone.Name)
	if err != nil {
		h.sendText(fmt.Sprintf("获取 %s 解析失败: %v", domain, err))
		return
	}
	if len(records) == 0 {
		h.sendText(fmt.Sprintf("域名 %s 在 %s 中没有解析记录。", domain, account.Label))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【域名解析记录】\n域名: %s\n账号: %s\n\n", zone.Name, account.Label))
	for _, r := range records {
		proxied := "off"
		if r.Proxied != nil && *r.Proxied {
			proxied = "on"
		}
		sb.WriteString(fmt.Sprintf("%s %s → %s (代理:%s)\n", r.Type, r.Name, r.Content, proxied))
	}
	h.sendText(sb.String())
}

func (h *CommandHandler) handleGetNSCommand(args []string) {
	if len(args) < 1 {
		h.sendText("用法: /getns <domain.com>")
		return
	}
	domain := strings.ToLower(args[0])

	if account, zone, err := h.findZone(domain); err == nil {
		h.sendText(fmt.Sprintf("域名 %s 已在账号 %s 下，NS: %s", zone.Name, account.Label, strings.Join(zone.NameServers, ", ")))
		return
	}

	account := h.defaultAccount()
	if account == nil {
		h.sendText("未配置可用的 Cloudflare 账号，无法添加域名。")
		return
	}

	zone, err := h.CFClient.CreateZone(context.Background(), *account, domain)
	if err != nil {
		h.sendText(fmt.Sprintf("添加域名失败: %v,%s---%s", err, domain, account.Label))
		return
	}

	h.sendText(fmt.Sprintf("已将 %s 添加到账号 %s，NS 请设置为: %s", zone.Name, account.Label, strings.Join(zone.NameServers, ", ")))
}

func (h *CommandHandler) handleStatusCommand(args []string) {
	if len(args) < 1 {
		h.sendText("用法: /status <domain.com>")
		return
	}
	domain := strings.ToLower(args[0])

	account, zone, err := h.findZone(domain)
	if err != nil {
		if errors.Is(err, cfclient.ErrZoneNotFound) {
			h.sendText(fmt.Sprintf("域名 %s 不存在于 Cloudflare。", domain))
			return
		}
		h.sendText(fmt.Sprintf("查询状态失败: %v", err))
		return
	}

	status := fmt.Sprintf("域名: %s\n账号: %s\n状态: %s\nPaused: %v", zone.Name, account.Label, zone.Status, zone.Paused)
	h.sendText(status)
}

func (h *CommandHandler) handleSetDNSCommand(args []string) {
	if len(args) < 4 {
		h.sendText("用法: /setdns <type> <sub.domain.com> <target> <on|off>")
		h.sendText("示例: /setdns cname abc.example.com k8s-internat-tgnlbdir-cebb795ee4-10a8cd291bfbaf76.elb.us-west-1.amazonaws.com on")
		return
	}

	recordType := strings.ToUpper(args[0])
	name := args[1]
	content := args[2]
	proxied := strings.EqualFold(args[3], "on")
	domain := deriveDomainFromName(name)
	if len(args) >= 5 {
		domain = strings.ToLower(args[4])
	}

	if domain == "" {
		h.sendText("请使用完整的域名或明确指定要操作的域名。")
		return
	}

	account, zone, err := h.findZone(domain)
	if err != nil {
		if errors.Is(err, cfclient.ErrZoneNotFound) {
			h.sendText(fmt.Sprintf("域名 %s 不存在于 Cloudflare，无法设置解析。", domain))
			return
		}
		h.sendText(fmt.Sprintf("查询域名失败: %v", err))
		return
	}

	params := cfclient.DNSRecordParams{Type: recordType, Name: name, Content: content, Proxied: proxied}
	record, err := h.CFClient.UpsertDNSRecord(context.Background(), *account, zone.Name, params)
	if err != nil {
		h.sendText(fmt.Sprintf("设置解析失败: %v", err))
		return
	}

	proxyStatus := "off"
	if record.Proxied != nil && *record.Proxied {
		proxyStatus = "on"
	}
	h.sendText(fmt.Sprintf("已在账号 %s 设置记录: %s %s → %s (代理:%s)", account.Label, record.Type, record.Name, record.Content, proxyStatus))
}

func (h *CommandHandler) findZone(domain string) (*config.CF, cfclient.ZoneDetail, error) {
	var lastErr error
	for i := range h.Accounts {
		acc := h.Accounts[i]
		zone, err := h.CFClient.GetZoneDetails(context.Background(), acc, domain)
		if err != nil {
			if errors.Is(err, cfclient.ErrZoneNotFound) {
				lastErr = err
				continue
			}
			return nil, cfclient.ZoneDetail{}, err
		}
		return &acc, zone, nil
	}
	if lastErr == nil {
		lastErr = cfclient.ErrZoneNotFound
	}
	return nil, cfclient.ZoneDetail{}, lastErr
}

// defaultAccount 随机返回一个 Cloudflare 账号配置
func (h *CommandHandler) defaultAccount() *config.CF {
	if len(h.Accounts) == 0 {
		return nil
	}
	idx := rand.Intn(len(h.Accounts))
	return &h.Accounts[idx]
}

func (h *CommandHandler) sendText(msg string) {
	_ = h.Sender.Send(context.Background(), msg)
}

func deriveDomainFromName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return ""
}
