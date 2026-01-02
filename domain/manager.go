package domain

import (
	"bufio"
	"context"
	"log"
	"os"
	"strings"

	"DomainC/cfclient"
	"DomainC/config"
	"DomainC/tools"
)

type DomainSource struct {
	Domain string
	Source string
	Expiry string
	IsCF   bool
}

// fetchActiveDomainsFromCF 已迁移到 cfclient 包，由 cfclient.Client 提供实现。

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

type Service struct {
	CF cfclient.Client
}

func NewService(cf cfclient.Client) *Service {
	if cf == nil {
		cf = cfclient.NewClient()
	}
	return &Service{CF: cf}
}

// CollectAll 从 CF 账户和本地文件收集所有域名
func (s *Service) CollectAll(accounts []config.CF, files []string) ([]DomainSource, error) {
	var out []DomainSource

	for _, acc := range accounts {
		doms, err := s.CF.FetchActiveDomains(context.Background(), acc)
		if err != nil {
			log.Printf("[%s] 获取域名失败: %v", acc.Label, err)
			continue
		}
		for _, d := range doms {
			out = append(out, DomainSource{
				Domain: d.Domain,
				Source: d.Source,
				IsCF:   d.IsCF,
			})
		}
	}

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
	return out, nil
}
