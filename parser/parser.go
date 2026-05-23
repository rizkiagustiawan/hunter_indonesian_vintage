package parser

import (
	"fbm-vintage-monitor/db"
	"log"
	"regexp"
	"strconv"
	"time"
)

var (
	// More flexible patterns to capture ID and Title even if attributes are reordered
	patternA = regexp.MustCompile(`"id":"(\d{10,})"[^}]*?"marketplace_listing_title":"([^"]+)"`)
	patternB = regexp.MustCompile(`"marketplace_listing_title":"([^"]+)"[^}]*?"id":"(\d{10,})"`)
	pricePat = regexp.MustCompile(`"amount":"(\d+)"`)

	// Backup pattern for simpler HTML/JSON structures
	backupIDPat = regexp.MustCompile(`"listing_id":"(\d+)"`)
)

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func extractPrice(idMatchStart int, html string) int {
	// Search in a larger 1000-char window for 2026 layouts
	windowStart := max(0, idMatchStart-1000)
	windowEnd := min(len(html), idMatchStart+1000)
	windowStr := html[windowStart:windowEnd]

	bestPrice := 0
	minDist := 2000

	matches := pricePat.FindAllStringSubmatchIndex(windowStr, -1)
	for _, match := range matches {
		globalPmStart := windowStart + match[0]
		dist := abs(globalPmStart - idMatchStart)
		if dist < minDist {
			minDist = dist
			p, _ := strconv.Atoi(windowStr[match[2]:match[3]])
			bestPrice = p
		}
	}
	return bestPrice
}

func ParseListings(html, city, keyword string, priceMax int) []db.Listing {
	type pair struct {
		title string
		price int
	}
	pairs := make(map[string]pair)

	matchesA := patternA.FindAllStringSubmatchIndex(html, -1)
	for _, m := range matchesA {
		lid := html[m[2]:m[3]]
		title := html[m[4]:m[5]]
		if _, exists := pairs[lid]; !exists {
			price := extractPrice(m[2], html)
			pairs[lid] = pair{title, price}
		}
	}

	matchesB := patternB.FindAllStringSubmatchIndex(html, -1)
	for _, m := range matchesB {
		title := html[m[2]:m[3]]
		lid := html[m[4]:m[5]]
		if _, exists := pairs[lid]; !exists {
			price := extractPrice(m[4], html)
			pairs[lid] = pair{title, price}
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var listings []db.Listing

	for lid, p := range pairs {
		if p.price < 1000 {
			continue
		}
		if priceMax > 0 && p.price > priceMax {
			continue
		}

		listings = append(listings, db.Listing{
			ID:       lid,
			Title:    p.title,
			PriceIDR: p.price,
			City:     city,
			Keyword:  keyword,
			URL:      "https://www.facebook.com/marketplace/item/" + lid,
			FoundAt:  now,
		})
	}

	log.Printf("Parsed %d valid listings for '%s' in %s", len(listings), keyword, city)
	return listings
}
