package domain

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"DomainC/config"
	"DomainC/tools"

	"github.com/cloudflare/cloudflare-go"
)

type DomainSource struct {
	Domain string
	Source string
	Expiry string
	IsCF   bool
}

func fetchActiveDomains(acc config.CF) ([]DomainSource, error) {
	api, err := cloudflare.NewWithAPIToken(acc.APIToken)
	if err != nil {
		return nil, fmt.Errorf("初始化 Cloudflare 客户端失败 [%s]: %v", acc.Label, err)
	}

	zones, err := api.ListZonesContext(
		context.Background(),
		cloudflare.WithZoneFilters("", "", "active"),
	)
	if err != nil {
		return nil, fmt.Errorf("获取域名失败 [%s]: %v", acc.Label, err)
	}

	var domains []DomainSource
	for _, zone := range zones.Result {
		if !zone.Paused {
			domains = append(domains, DomainSource{
				Domain: zone.Name,
				Source: acc.Label,
				IsCF:   true,
			})
		}
	}
	return domains, nil
}

func GetAll(accounts []config.CF, files []string) []DomainSource {
	var out []DomainSource

	// 从 Cloudflare 账户拉取
	for _, acc := range accounts {
		domains, err := fetchActiveDomains(acc)
		if err != nil {
			log.Printf("[%s] 获取域名失败: %v", acc.Label, err)
			continue
		}
		out = append(out, domains...)
	}

	// 本地文件读取
	for _, f := range files {
		fd, err := os.Open(f)
		if err != nil {
			log.Printf("无法打开文件 %s: %v", f, err)
			continue
		}
		sc := bufio.NewScanner(fd)
		for sc.Scan() {
			d := strings.TrimSpace(sc.Text())
			if d != "" {
				out = append(out, DomainSource{Domain: d, Source: f, IsCF: false})
			}
		}
		if err := sc.Err(); err != nil {
			log.Printf("读取文件 %s 时出错: %v", f, err)
		}
		fd.Close()
	}
	return out
}

func DaysUntil(expiry string) (int, error) {
	return tools.DaysUntilExpiry(expiry)
}

func CheckWhois(domain string) string {
	return tools.CheckWhois(domain)
}

func ParseExpiry(whois string) string {
	return tools.ExtractExpiry(whois)
}

func SaveExpiring(domains []DomainSource, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, d := range domains {
		_, _ = w.WriteString(d.Domain + "|" + d.Source + "|" + d.Expiry + "\n")
	}
	return w.Flush()
}
