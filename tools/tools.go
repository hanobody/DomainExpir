package tools

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/likexian/whois"
	"github.com/openrdap/rdap"
)

func ExtractExpiry(result string) string {
	// 支持常见的到期日期格式
	patterns := []string{
		`(?i)(expiration|expiry|expires on)[^:]*[:\s]+([0-9]{4}-[0-9]{2}-[0-9]{2})`,
		`(?i)(expiration|expiry|expires on)[^:]*[:\s]+([0-9]{2}-[A-Za-z]{3}-[0-9]{4})`,
		`(?i)(expiration|expiry|expires on)[^:]*[:\s]+([A-Za-z]{3} [0-9]{2}, [0-9]{4})`,
		`(?i)(expiration|expiry|expires on)[^:]*[:\s]+([0-9T:\-\.Z]+)`,
	}

	layouts := []string{
		"2006-01-02",
		"02-Jan-2006",
		"Jan 02, 2006",
		time.RFC3339,           // 2025-08-30T03:46:27.917Z
		"2006-01-02T15:04:05Z", // fallback
		"2006-01-02 15:04:05",  // common alt format
	}

	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		match := re.FindStringSubmatch(result)
		if len(match) >= 3 {
			dateStr := strings.TrimSpace(match[2])
			for _, layout := range layouts {
				if t, err := time.Parse(layout, dateStr); err == nil {
					return t.Format("2006-01-02")
				}
			}
		}
	}
	return result
}

type Button struct {
	Text         string
	CallbackData string
}

func CheckWhois(domain string) string {
	client := &rdap.Client{}

	d, err := client.QueryDomain(domain)
	if err == nil {
		for _, event := range d.Events {
			if event.Action == "expiration" {
				return fmt.Sprintf("%s: RDAP Expiration Date: %s", domain, event.Date)
			}
		}
	}
	result, err := whois.Whois(domain)
	if err != nil {
		return fmt.Sprintf("%s 查询失败:  WHOIS错误: %v", domain, err)
	}

	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(strings.ToLower(line), "expir") {
			return fmt.Sprintf("%s: %s", domain, line)
		}
	}

	return fmt.Sprintf("[%s]%s: ", domain, result)
}
func DaysUntilExpiry(expiry string) (int, error) {
	expiryTime, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return -1, fmt.Errorf("解析到期日期失败: %v", err)
	}
	days := int(time.Until(expiryTime).Hours() / 24)
	return days, nil
}
