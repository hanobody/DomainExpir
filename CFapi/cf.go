package cfapi

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudflare/cloudflare-go"
)

func GetAccountByLabel(label string) *CFAccount {
	accounts := []CFAccount{
		{Email: "fl881396@gmail.com", APIKey: "339d5f62b17167c524bd316376fb39e593436", Label: "FL"},
		{Email: "vip535018@gmail.com", APIKey: "7079691a61134fcff627a13d3edad30a3609b", Label: "VIP"},
		{Email: "twwin6688@gmail.com", APIKey: "ecebf3bd362a123dbc4bbcde75d688825479c", Label: "TW"},
		{Email: "ffds168168@gmail.com", APIKey: "268a00f38225a428be36b48117a3d89aea4b3", Label: "FFDS"},
	}
	for _, acc := range accounts {
		if acc.Label == label {
			return &acc
		}
	}
	return nil
}

type CFAccount struct {
	Email  string
	APIKey string
	Label  string
}

type DomainSource struct {
	Domain string
	Source string
	Expiry string
	IsCF   bool
}

func DeleteCloudflareDomain(account CFAccount, domain string) error {
	api, err := cloudflare.New(account.APIKey, account.Email)
	if err != nil {
		return fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	// 获取 Zone ID
	zones, err := api.ListZonesContext(
		context.Background(),
		cloudflare.WithZoneFilters(domain, "", ""),
	)
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

	log.Printf("域名 %s 已删除成功", domain)
	return nil
}
func ListCloudflareDNSRecords(account CFAccount, domain string) ([]cloudflare.DNSRecord, error) {
	api, err := cloudflare.New(account.APIKey, account.Email)
	if err != nil {
		return nil, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	// 获取 Zone ID
	zones, err := api.ListZonesContext(
		context.Background(),
		cloudflare.WithZoneFilters(domain, "", ""),
	)
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

func PauseCloudflareDomain(account CFAccount, domain string, pauseString string) error {
	api, err := cloudflare.New(account.APIKey, account.Email)
	if err != nil {
		return fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	// 获取 Zone ID
	zones, err := api.ListZonesContext(
		context.Background(),
		cloudflare.WithZoneFilters(domain, "", ""),
	)
	if err != nil {
		return fmt.Errorf("获取 Zone 失败: %v", err)
	}
	if len(zones.Result) == 0 {
		return fmt.Errorf("未找到域名 %s", domain)
	}
	zoneID := zones.Result[0].ID
	paused := false
	if pauseString == "yes" {
		paused = true
	}
	_, err = api.EditZone(context.Background(), zoneID, cloudflare.ZoneOptions{Paused: &paused})
	if err != nil {
		return fmt.Errorf("暂停域名失败: %v", err)
	}

	log.Printf("域名 %s-----%s 已暂停成功", domain, account.Label)
	return nil
}

func fetchActiveDomains(account CFAccount) ([]DomainSource, error) {
	api, err := cloudflare.New(account.APIKey, account.Email)
	if err != nil {
		return nil, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", account.Label, err)
	}

	// 获取状态为 active 的域名
	zones, err := api.ListZonesContext(
		context.Background(),
		cloudflare.WithZoneFilters("", "", "active"),
	)
	if err != nil {
		return nil, fmt.Errorf("获取域名失败 [%s]: %v", account.Label, err)
	}

	var domains []DomainSource
	for _, zone := range zones.Result {
		// 只保留非 paused 的域名
		if !zone.Paused {
			domains = append(domains, DomainSource{
				Domain: zone.Name,
				Source: account.Label,
				IsCF:   true,
			})
		}
	}
	return domains, nil
}

func GetAllDomains(accounts []CFAccount, filePaths []string) []DomainSource {
	var allDomains []DomainSource

	// 来自 Cloudflare 的域名
	for _, acc := range accounts {
		domains, err := fetchActiveDomains(acc)
		if err != nil {
			log.Printf("[%s] 获取域名失败: %v\n", acc.Label, err)
			continue
		}
		allDomains = append(allDomains, domains...)
	}

	// 来自本地文件的域名
	for _, filePath := range filePaths {
		file, err := os.Open(filePath)
		if err != nil {
			log.Printf("无法打开文件 %s: %v\n", filePath, err)
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			domain := strings.TrimSpace(scanner.Text())
			if domain != "" {
				allDomains = append(allDomains, DomainSource{
					Domain: domain,
					Source: filePath,
					IsCF:   false,
				})
			}
		}
		file.Close()
		if err := scanner.Err(); err != nil {
			log.Printf("读取文件 %s 时发生错误: %v\n", filePath, err)
		}
	}

	return allDomains
}

func SaveExpiringDomainsToFile(domains []DomainSource, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, ds := range domains {
		line := fmt.Sprintf("%s|%s|%s\n", ds.Domain, ds.Source, ds.Expiry)
		if _, err := writer.WriteString(line); err != nil {
			return fmt.Errorf("写入文件失败: %v", err)
		}
	}

	return writer.Flush()
}
