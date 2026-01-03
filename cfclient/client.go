package cfclient

import (
	"context"
	"fmt"
	"time"

	"DomainC/config"

	cloudflare "github.com/cloudflare/cloudflare-go"
)

// DomainInfo 是 cfclient 层的域名描述，避免直接依赖 domain 包
type DomainInfo struct {
	Domain string
	Source string
	IsCF   bool
	Status string
	Paused bool
}

// Client 定义了 Cloudflare 相关操作的抽象接口
type Client interface {
	FetchAllDomains(ctx context.Context, account config.CF) ([]DomainInfo, error)
	ListDNSRecords(ctx context.Context, account config.CF, domain string) ([]cloudflare.DNSRecord, error)
	PauseDomain(ctx context.Context, account config.CF, domain string, pause bool) error
	DeleteDomain(ctx context.Context, account config.CF, domain string) error
}

type apiClient struct{}

// NewClient 返回默认的 Cloudflare API 客户端实现
func NewClient() Client {
	return &apiClient{}
}

// DeleteDomain 从 Cloudflare 删除 zone
func (c *apiClient) DeleteDomain(ctx context.Context, account config.CF, domain string) error {
	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zones, err := api.ListZonesContext(ctx, cloudflare.WithZoneFilters(domain, "", ""))
	if err != nil {
		return fmt.Errorf("获取 Zone 失败: %v", err)
	}
	if len(zones.Result) == 0 {
		return fmt.Errorf("未找到域名 %s", domain)
	}

	zoneID := zones.Result[0].ID
	_, err = api.DeleteZone(ctx, zoneID)
	if err != nil {
		return fmt.Errorf("删除域名失败: %v", err)
	}
	return nil
}

// 为兼容旧调用，保留包级函数，转发到默认客户端
func DeleteDomain(account config.CF, domain string) error {
	return NewClient().DeleteDomain(context.Background(), account, domain)
}

// ListDNSRecords 返回指定域名的解析记录
func (c *apiClient) ListDNSRecords(ctx context.Context, account config.CF, domain string) ([]cloudflare.DNSRecord, error) {
	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return nil, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zones, err := api.ListZonesContext(ctx, cloudflare.WithZoneFilters(domain, "", ""))
	if err != nil {
		return nil, fmt.Errorf("获取 Zone 失败: %v", err)
	}
	if len(zones.Result) == 0 {
		return nil, fmt.Errorf("未找到域名 %s", domain)
	}

	zoneID := zones.Result[0].ID
	records, _, err := api.ListDNSRecords(
		ctx,
		cloudflare.ZoneIdentifier(zoneID),
		cloudflare.ListDNSRecordsParams{},
	)
	if err != nil {
		return nil, fmt.Errorf("获取 DNS 记录失败: %v", err)
	}
	return records, nil
}

func ListDNSRecords(account config.CF, domain string) ([]cloudflare.DNSRecord, error) {
	return NewClient().ListDNSRecords(context.Background(), account, domain)
}

// PauseDomain 设置 zone 的 paused 状态
func (c *apiClient) PauseDomain(ctx context.Context, account config.CF, domain string, pause bool) error {
	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zones, err := api.ListZonesContext(ctx, cloudflare.WithZoneFilters(domain, "", ""))
	if err != nil {
		return fmt.Errorf("获取 Zone 失败: %v", err)
	}
	if len(zones.Result) == 0 {
		return fmt.Errorf("未找到域名 %s", domain)
	}

	zoneID := zones.Result[0].ID
	_, err = api.EditZone(
		ctx,
		zoneID,
		cloudflare.ZoneOptions{Paused: &pause},
	)
	if err != nil {
		return fmt.Errorf("设置 paused 失败: %v", err)
	}
	return nil
}

func PauseDomain(account config.CF, domain string, pause bool) error {
	return NewClient().PauseDomain(context.Background(), account, domain, pause)
}

func (c *apiClient) FetchAllDomains(ctx context.Context, account config.CF) ([]DomainInfo, error) {

	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return nil, fmt.Errorf(
			"初始化 Cloudflare 客户端失败 [%s]: %v",
			account.Label, err,
		)
	}

	zones, err := api.ListZonesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"获取域名失败 [%s]: %v",
			account.Label, err,
		)
	}

	out := make([]DomainInfo, 0, len(zones.Result))

	for _, z := range zones.Result {
		out = append(out, DomainInfo{
			Domain: z.Name,
			Source: account.Label,
			IsCF:   true,
			Status: z.Status,
			Paused: z.Paused,
		})
	}

	return out, nil
}

func FetchAllDomains(account config.CF) ([]DomainInfo, error) {
	return NewClient().FetchAllDomains(context.Background(), account)
}

// GetAccountByLabel 返回配置中与 label 匹配的 Cloudflare 账号指针，找不到则返回 nil
func GetAccountByLabel(label string) *config.CF {
	for i := range config.Cfg.CloudflareAccounts {
		if config.Cfg.CloudflareAccounts[i].Label == label {
			return &config.Cfg.CloudflareAccounts[i]
		}
	}
	return nil
}

func ensureTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, 30*time.Second)
}
