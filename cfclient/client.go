package cfclient

import (
	"context"
	"fmt"

	"DomainC/config"
	"DomainC/domain"

	cloudflare "github.com/cloudflare/cloudflare-go"
)

// DeleteDomain 从 Cloudflare 删除 zone
func DeleteDomain(account config.CF, domain string) error {
	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zones, err := api.ListZonesContext(context.Background(), cloudflare.WithZoneFilters(domain, "", ""))
	if err != nil {
		return fmt.Errorf("获取 Zone 失败: %v", err)
	}
	if len(zones.Result) == 0 {
		return fmt.Errorf("未找到域名 %s", domain)
	}

	zoneID := zones.Result[0].ID
	_, err = api.DeleteZone(context.Background(), zoneID)
	if err != nil {
		return fmt.Errorf("删除域名失败: %v", err)
	}
	return nil
}

// ListDNSRecords 返回指定域名的解析记录
func ListDNSRecords(account config.CF, domain string) ([]cloudflare.DNSRecord, error) {
	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return nil, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zones, err := api.ListZonesContext(context.Background(), cloudflare.WithZoneFilters(domain, "", ""))
	if err != nil {
		return nil, fmt.Errorf("获取 Zone 失败: %v", err)
	}
	if len(zones.Result) == 0 {
		return nil, fmt.Errorf("未找到域名 %s", domain)
	}

	zoneID := zones.Result[0].ID
	records, _, err := api.ListDNSRecords(
		context.Background(),
		cloudflare.ZoneIdentifier(zoneID),
		cloudflare.ListDNSRecordsParams{},
	)
	if err != nil {
		return nil, fmt.Errorf("获取 DNS 记录失败: %v", err)
	}
	return records, nil
}

// PauseDomain 设置 zone 的 paused 状态
func PauseDomain(account config.CF, domain string, pause bool) error {
	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zones, err := api.ListZonesContext(context.Background(), cloudflare.WithZoneFilters(domain, "", ""))
	if err != nil {
		return fmt.Errorf("获取 Zone 失败: %v", err)
	}
	if len(zones.Result) == 0 {
		return fmt.Errorf("未找到域名 %s", domain)
	}

	zoneID := zones.Result[0].ID
	_, err = api.EditZone(
		context.Background(),
		zoneID,
		cloudflare.ZoneOptions{Paused: &pause},
	)
	if err != nil {
		return fmt.Errorf("设置 paused 失败: %v", err)
	}
	return nil
}

// FetchActiveDomains 返回账户下处于 active 且未 paused 的域名
func FetchActiveDomains(account config.CF) ([]domain.DomainSource, error) {
	api, err := cloudflare.NewWithAPIToken(account.APIToken)
	if err != nil {
		return nil, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	zones, err := api.ListZonesContext(context.Background(), cloudflare.WithZoneFilters("", "", "active"))
	if err != nil {
		return nil, fmt.Errorf("获取域名失败 [%s]: %v", account.Label, err)
	}

	var out []domain.DomainSource
	for _, z := range zones.Result {
		if !z.Paused {
			out = append(out, domain.DomainSource{
				Domain: z.Name,
				Source: account.Label,
				IsCF:   true,
			})
		}
	}
	return out, nil
}
