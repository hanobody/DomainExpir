package domain

import (
	"context"
	"log"

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

func DaysUntil(expiry string) (int, error) {
	return tools.DaysUntilExpiry(expiry)
}

func CheckWhois(domain string) string {
	return tools.CheckWhois(domain)
}

func ParseExpiry(whois string) (string, bool) {
	return tools.ExtractExpiry(whois)
}

type Service struct {
	CF   cfclient.Client
	Repo Repository
}

func NewService(cf cfclient.Client, r Repository) *Service {
	if cf == nil {
		cf = cfclient.NewClient()
	}
	return &Service{CF: cf, Repo: r}
}

func (s *Service) CollectAll(accounts []config.CF) ([]DomainSource, error) {
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

	if s.Repo != nil {
		sources, err := s.Repo.LoadSources()
		if err != nil {
			return nil, err
		}
		out = append(out, sources...)
	}

	return out, nil
}
