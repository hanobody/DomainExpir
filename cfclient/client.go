package cfclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
type ZoneDetail struct {
	ID          string
	Name        string
	NameServers []string
	Status      string
	Paused      bool
}

// Client 定义了 Cloudflare 相关操作的抽象接口
type Client interface {
	FetchAllDomains(ctx context.Context, account config.CF) ([]DomainInfo, error)
	ListDNSRecords(ctx context.Context, account config.CF, domain string) ([]cloudflare.DNSRecord, error)
	PauseDomain(ctx context.Context, account config.CF, domain string, pause bool) error
	DeleteDomain(ctx context.Context, account config.CF, domain string) error
	GetZoneDetails(ctx context.Context, account config.CF, domain string) (ZoneDetail, error)
	CreateZone(ctx context.Context, account config.CF, domain string) (ZoneDetail, error)
	UpsertDNSRecord(ctx context.Context, account config.CF, domain string, params DNSRecordParams) (cloudflare.DNSRecord, error)
}

type apiClient struct{}

// NewClient 返回默认的 Cloudflare API 客户端实现
func NewClient() Client {
	return &apiClient{}
}

// ErrZoneNotFound 在账户中未找到域名时返回
var ErrZoneNotFound = errors.New("zone not found")

// DNSRecordParams 描述需要创建或更新的解析记录
type DNSRecordParams struct {
	Type    string
	Name    string
	Content string
	Proxied bool
	TTL     int
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
		return fmt.Errorf("%w: %s", ErrZoneNotFound, domain)
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
		return nil, fmt.Errorf("%w: %s", ErrZoneNotFound, domain)
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

// GetZoneDetails 根据域名查找 Cloudflare zone
func (c *apiClient) GetZoneDetails(ctx context.Context, account config.CF, domain string) (ZoneDetail, error) {
	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return ZoneDetail{}, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zones, err := api.ListZonesContext(ctx, cloudflare.WithZoneFilters(domain, "", ""))
	if err != nil {
		return ZoneDetail{}, fmt.Errorf("获取 Zone 失败: %v", err)
	}
	if len(zones.Result) == 0 {
		return ZoneDetail{}, fmt.Errorf("%w: %s", ErrZoneNotFound, domain)
	}

	zone := zones.Result[0]
	return ZoneDetail{
		ID:          zone.ID,
		Name:        zone.Name,
		NameServers: zone.NameServers,
		Status:      zone.Status,
		Paused:      zone.Paused,
	}, nil
}

// CreateZone 将域名添加到 Cloudflare
func (c *apiClient) CreateZone(ctx context.Context, account config.CF, domain string) (ZoneDetail, error) {
	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return ZoneDetail{}, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	cfAccount := cloudflare.Account{ID: account.AccountID, Name: account.Label}
	zone, err := api.CreateZone(ctx, domain, false, cfAccount, "full")
	if err != nil {
		return ZoneDetail{}, fmt.Errorf("创建域名失败: %v", err)
	}

	return ZoneDetail{
		ID:          zone.ID,
		Name:        zone.Name,
		NameServers: zone.NameServers,
		Status:      zone.Status,
		Paused:      zone.Paused,
	}, nil
}

// UpsertDNSRecord 创建或更新解析记录
func (c *apiClient) UpsertDNSRecord(ctx context.Context, account config.CF, domain string, params DNSRecordParams) (cloudflare.DNSRecord, error) {
	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return cloudflare.DNSRecord{}, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zone, err := c.GetZoneDetails(ctx, account, domain)
	if err != nil {
		return cloudflare.DNSRecord{}, err
	}

	ttl := params.TTL
	if ttl == 0 {
		ttl = 1
	}
	proxied := params.Proxied

	searchParams := cloudflare.ListDNSRecordsParams{Type: params.Type}
	if params.Name == "" {
		searchParams.Name = zone.Name
	} else if strings.Contains(params.Name, zone.Name) {
		searchParams.Name = params.Name
	} else {
		searchParams.Name = fmt.Sprintf("%s.%s", params.Name, zone.Name)
	}

	existing, _, err := api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zone.ID), searchParams)
	if err != nil {
		return cloudflare.DNSRecord{}, fmt.Errorf("查询解析记录失败: %v", err)
	}

	if len(existing) > 0 {
		target := existing[0]
		record, err := api.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zone.ID), cloudflare.UpdateDNSRecordParams{
			ID:      target.ID,
			Type:    params.Type,
			Name:    searchParams.Name,
			Content: params.Content,
			TTL:     ttl,
			Proxied: &proxied,
		})
		if err != nil {
			return cloudflare.DNSRecord{}, fmt.Errorf("更新解析记录失败: %v", err)
		}
		return record, nil
	}

	record, err := api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zone.ID), cloudflare.CreateDNSRecordParams{
		Type:    params.Type,
		Name:    searchParams.Name,
		Content: params.Content,
		TTL:     ttl,
		Proxied: &proxied,
	})
	if err != nil {
		return cloudflare.DNSRecord{}, fmt.Errorf("创建解析记录失败: %v", err)
	}

	return record, nil
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
