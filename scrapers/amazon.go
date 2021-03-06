package scrapers

import (
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"

	"encoding/json"

	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var _ = fmt.Println
var totalResult string = "totalResultCount"
var amazonURL = "https://www.amazon.com/s/ref=nb_sb_noss?field-keywords="

// Arguments for arguments type
type Arguments struct {
	Fulfillment  string  `json:"fulfillment"`
	Price        float64 `json:"price"`
	ReviewCount  int     `json:"reviewCount"`
	ReviewRating float64 `json:"reviewRating"`
	Sales        int     `json:"sales"`
}

// JungleJSON for jungle json type
type JungleJSON struct {
	Status         bool   `json:"status"`
	Message        string `json:"message"`
	EstSalesResult int    `json:"estSalesResult"`
}

// AmazonSearchResults scrape amazon site and get number of result
func AmazonSearchResults(query string) (numOfResults int) {
	c := colly.NewCollector()
	c.DisableCookies()
	extensions.RandomUserAgent(c)

	var err error
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Upgrade-Insecure-Requests", "1")
		log.Println("visiting ", r.URL.String())
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Println("retrying", err)
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		if !strings.Contains(e.Text, totalResult) {
			return
		}
		tar := strings.Split(e.Text, totalResult)[1]
		tar = strings.Split(tar, ",")[0]
		tar = strings.Split(tar, ":")[1]
		numOfResults, err = strconv.Atoi(tar)
		if err != nil {
			log.Println("error converting num of results to int", err)
			return
		}
	})

	err = c.Visit(amazonURL + url.QueryEscape(query))
	if err != nil {
		log.Println("error requesting amazon search count for phrase", query, err)
		log.Println("retrying")
		return AmazonSearchResults(query)
	}

	c.Wait()

	return numOfResults
}

// AmazonJungleScout scrapes amazon and get arguments result
func AmazonJungleScout(query string) []Arguments {
	// collectors
	c := colly.NewCollector()
	c.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/67.0.3396.87 Safari/537.36"
	c.SetRequestTimeout(10000000000000)
	c.SetProxy("209.126.120.13:9500")

	page := c.Clone()

	jungle := colly.NewCollector()
	jungle.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/67.0.3396.87 Safari/537.36"
	jungle.CacheDir = "jCache"

	var data []Arguments
	var _ = data
	// parsers
	c.OnHTML("li.s-result-item", func(e *colly.HTMLElement) {
		asin := e.Attr("data-asin")
		//fmt.Println(e.ChildText("h2"))
		page.Visit("https://www.amazon.com/dp/" + asin)
	})

	var fulfillment string
	page.OnHTML("#merchant-info", func(e *colly.HTMLElement) {
		//fmt.Println(e.Request.URL.String())
		if strings.Contains(e.Text, "Fulfilled by Amazon") {
			fulfillment = "FBA"
		} else {
			fulfillment = "AMZ"
		}

	})
	counter := 0
	page.OnHTML("html", func(e *colly.HTMLElement) {
		revRatingReg := regexp.MustCompile(`^[0-5]?\.?[0-9]`)
		priceReg := regexp.MustCompile(`(?i)new ?\(?[0-9]*?\) from \$([0-9.])*`)

		revCount := e.ChildText(".totalReviewCount")
		revCount = strings.Replace(revCount, ",", "", -1)
		revRating := revRatingReg.FindString(e.ChildText(".arp-rating-out-of-text"))
		priceFind := priceReg.FindString(e.Text)
		spPriceFind := strings.Split(priceFind, "$")
		price := spPriceFind[(len(spPriceFind) - 1)]
		//fmt.Println(revCount, revRating, price, fulfillment)

		reRang := regexp.MustCompile(`#[0-9,]* in .+`)
		var rank, category string
		if reRang.MatchString(e.Text) {
			sp := strings.Split(reRang.FindString(e.Text), " ")
			if len(sp) > 2 {
				rank = strings.Replace(sp[0], "#", "", -1)
				rank = strings.Replace(rank, ",", "", -1)
				category = strings.Join(sp[2:], " ")
				spc := strings.Split(category, "(")
				if len(spc) > 0 {
					category = strings.TrimSpace(spc[0])
				}
			}
		}

		priceFloat, err := strconv.ParseFloat(price, 64)
		if err != nil {
			log.Println("error converting price to float", err)
		}
		revCountInt, err := strconv.Atoi(revCount)
		if err != nil {
			log.Print("error converting revCount to int", err)
		}
		revRatingFloat, err := strconv.ParseFloat(revRating, 64)
		data = append(data, Arguments{
			Fulfillment:  fulfillment,
			Price:        priceFloat,
			ReviewCount:  revCountInt,
			ReviewRating: revRatingFloat,
		})
		counter++
		err = jungle.Visit(`https://junglescoutpro.herokuapp.com/api/v1/est_sales?store=us&rank=` + rank + `&category=` + url.QueryEscape(category) + `&dailyToken=tPrDggSZDJZzBi80e3sKug==`)
		if err != nil {
			log.Println("jungle err", err)
		}
	})

	jungle.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Referer", "https://www.amazon.com/s/field-keywords="+url.QueryEscape(query))
	})

	jungle.OnResponse(func(r *colly.Response) {
		jungleData := JungleJSON{}
		json.Unmarshal(r.Body, &jungleData)
		data[counter-1].Sales = jungleData.EstSalesResult
	})

	err := c.Visit("https://www.amazon.com/s/field-keywords=" + url.QueryEscape(query))
	if err != nil {
		log.Println("error starting scrape", err)
	}
	page.Wait()

	return data
}
