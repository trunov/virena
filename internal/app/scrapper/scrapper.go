package scraper

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
)

type Scraper struct {
	collector *colly.Collector
}

func NewScraper() *Scraper {
	// Create a new collector
	c := colly.NewCollector()

	c.SetCookies("https://originaalosad.ee", []*http.Cookie{
		{Name: "login[email]", Value: "virenaauto%40gmail.com"},
		{Name: "login[password]", Value: "31cceda1d0c20c40c7ede37fc871b8e1"},
	})

	// Initialize and return the scraper with the collector
	return &Scraper{
		collector: c,
	}
}

func (s *Scraper) GetOriginaalosadPriceData(url string) (float64, error) {
	var originaalosadPrice float64

	s.collector.OnHTML(".productsList", func(e *colly.HTMLElement) {
		// Extract the price element
		priceElement := e.DOM.Find(".productPrice .pnvt").First()

		// Extract the price value from the element
		priceString := priceElement.Text()
		priceString = strings.ReplaceAll(priceString, ",", "")

		var err error
		originaalosadPrice, err = strconv.ParseFloat(priceString, 64)
		if err != nil {
			log.Printf("Failed to parse price: %v", err)
			return
		}
	})

	// Start the scraping process
	err := s.collector.Visit(url)
	if err != nil {
		return 0, fmt.Errorf("failed to scrape product data: %v", err)
	}

	// Return the fetched product data
	return originaalosadPrice, nil
}
