package app

import (
	"context"
	"log"
	"time"

	"DomainC/domain"
	"DomainC/tools"
)

type WhoisClient interface {
	Query(ctx context.Context, domain string) (string, error)
}

type ExpiryCheckerService struct {
	Whois        WhoisClient
	Repo         domain.Repository
	AlertWithin  time.Duration
	RateLimit    time.Duration
	QueryTimeout time.Duration
}

func (c *ExpiryCheckerService) Check(ctx context.Context, domains []domain.DomainSource) ([]domain.DomainSource, error) {
	if c.Whois == nil {
		return nil, ErrMissingDependencies
	}
	if c.AlertWithin == 0 {
		c.AlertWithin = 24 * time.Hour
	}
	var ticker *time.Ticker
	if c.RateLimit > 0 {
		ticker = time.NewTicker(c.RateLimit)
		defer ticker.Stop()
	}

	var expiring []domain.DomainSource
	for i, ds := range domains {
		if i > 0 && ticker != nil {
			select {
			case <-ctx.Done():
				return expiring, ctx.Err()
			case <-ticker.C:
			}
		}

		lookupCtx := ctx
		cancel := func() {}
		if c.QueryTimeout > 0 {
			lookupCtx, cancel = context.WithTimeout(ctx, c.QueryTimeout)
		}
		result, err := c.Whois.Query(lookupCtx, ds.Domain)
		cancel()
		if err != nil {
			log.Printf("WHOIS 查询失败 (%s): %v", ds.Domain, err)
			continue
		}

		expiry := tools.ExtractExpiry(result)
		expiryTime, err := time.Parse("2006-01-02", expiry)
		if err != nil {
			log.Printf("解析到期时间失败 [%s]: %v", ds.Domain, err)
			continue
		}

		if time.Until(expiryTime) <= c.AlertWithin {
			ds.Expiry = expiry
			expiring = append(expiring, ds)
		}
	}

	if c.Repo != nil {
		if err := c.Repo.SaveExpiring(expiring); err != nil {
			return expiring, err
		}
	}
	return expiring, nil
}
