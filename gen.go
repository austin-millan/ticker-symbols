package main

//go:generate go run gen.go

import (
	"bytes"
	"encoding/csv"
	"github.com/jlaffaye/ftp"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	nasdaqAddr = "ftp.nasdaqtrader.com:21"
	symbolDir = "SymbolDirectory"
	anonymousUser = "anonymous"
	anonymousPassword = "gitlab.com/brokerage-api/ticker-symbols"
	maxSymbolNameLength = 40
)

type ret struct {
	V   CompanyListing
	Err error
}

func fetchFtpFile(fname string) ([]byte, error) {
	var err error
	var c *ftp.ServerConn
	if c, err = ftp.Dial(nasdaqAddr, ftp.DialWithTimeout(60*time.Second), ftp.DialWithDisabledEPSV(false)); err != nil {
		log.Fatal(err)
		return nil, err
	}
	if err = c.Login(anonymousUser, anonymousPassword); err != nil {
		log.Fatal(err)
	}
	if err = c.ChangeDir(symbolDir); err != nil {
		return nil, err
	}
	r, err := c.Retr(fname)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	buf, err := ioutil.ReadAll(r)
	// Do something with the FTP conn
	if err = c.Quit(); err != nil {
		log.Fatal(err)
	}
	return buf, nil
}

// parse a csv file and return an array of resources
func parse(r io.Reader) chan ret {
	c := make(chan ret, 0)
	go func() {
		defer close(c)
		rd := csv.NewReader(r)
		rd.Comma = rune('|')
		var header []string
		header, err := rd.Read()
		if err != nil {
			c <- ret{CompanyListing{}, err}
		}

		e := CompanyListing{}
		et := reflect.TypeOf(e)
		var headers = make(map[string]int, et.NumField())
		for i := 0; i < et.NumField(); i++ {
			headers[et.Field(i).Name] = func(element string, array []string) int {
				for k, v := range array {
					if v == element {
						return k
					}
				}
				return -1
			}(et.Field(i).Tag.Get("csv"), header)
		}
		for {
			var e = CompanyListing{}
			record, err := rd.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				c <- ret{CompanyListing{}, err}
			}
			for h, i := range headers {
				if i == -1 {
					continue
				}
				elem := reflect.ValueOf(&e).Elem()
				field := elem.FieldByName(h)
				if field.CanSet() {
					switch field.Type().Name() {
					case "float64":
						a, _ := strconv.ParseFloat(record[i], 64)
						field.Set(reflect.ValueOf(a))
					case "Time":
						a, _ := time.Parse("2006-01-02T00:00:00Z", record[i])
						field.Set(reflect.ValueOf(a))
					default:
						if i > len(record)-1 {
							continue
						}
						field.Set(reflect.ValueOf(record[i]))
					}
				}
			}
			c <- ret{e, nil}
		}
	}()
	return c
}

// CompanyListing holds a structure
// Symbol|Security Name|Market Category|Test Issue|Financial Status|Round Lot Size|ETF|NextShares

// ACT Symbol|Security Name|Exchange|CQS Symbol|ETF|Round Lot Size|Test Issue|NASDAQ Symbol
type CompanyListing struct {
	// The one to four or five character identifier for each NASDAQ-listed security.
	Symbol   string `csv:"Symbol"`
	// Identifier of the security used to in various NASDAQ connectivity protocols and NASDAQ market data feeds.
	// Typical identifiers have 1-5 character root symbol and then 1-3 characters for suffixes. Allow up to 14 characters.
	NasdaqSymbol string `csv:"NASDAQ Symbol"`
	// The listing stock exchange or market of a security.
	// Allowed values are:
	//
	//    A = NYSE MKT
	//    N = New York Stock Exchange (NYSE)
	//    P = NYSE ARCA
	//    Z = BATS Global Markets (BATS)
	//    V = Investors' Exchange, LLC (IEXG)
	Exchange string `csv:"Exchange"`
	// Identifier of the security used to disseminate data via the SIAC Consolidated Quotation System (CQS) and
	// Consolidated Tape System (CTS) data feeds. Typical identifiers have 1-5 character root symbol and then 1-3
	// characters for suffixes. Allow up to 14 characters.
	CQSSymbol string `csv:"CQS Symbol"`
	// Identifier for each security used in ACT and CTCI connectivity protocol. Typical identifiers have 1-5 character
	// root symbol and then 1-3 characters for suffixes. Allow up to 14 characters.
	ACTSymbol   string `csv:"ACT Symbol"`
	// The name of the security including additional information, if applicable. Examples are security type
	// (common stock, preferred stock, etc.) or class (class A or B, etc.). Allow up to 255 characters.
	SecurityName  string `csv:"Security Name"`
	// The category assigned to the issue by NASDAQ based on Listing Requirements. Values:
	//
	//    Q = NASDAQ Global Select MarketSM
	//    G = NASDAQ Global MarketSM
	//    S = NASDAQ Capital Market
	MarketCategory string `csv:"Market Category"`
	// Indicates whether the security is a test security.
	//    Y = Yes, it is a test issue.
	//    N = No, it is not a test issue.
	TestIssue string `csv:"Test Issue"`
	//  Indicates when an issuer has failed to submit its regulatory filings on a timely basis, has failed to meet NASDAQ's continuing listing standards, and/or has filed for bankruptcy. Values include:
	//
	//    D = Deficient: Issuer Failed to Meet NASDAQ Continued Listing Requirements
	//    E = Delinquent: Issuer Missed Regulatory Filing Deadline
	//    Q = Bankrupt: Issuer Has Filed for Bankruptcy
	//    N = Normal (Default): Issuer Is NOT Deficient, Delinquent, or Bankrupt.
	//    G = Deficient and Bankrupt
	//    H = Deficient and Delinquent
	//    J = Delinquent and Bankrupt
	//    K = Deficient, Delinquent, and Bankrupt
	FinancialStatus string `csv:"Financial Status"`
	// Indicates the number of shares that make up a round lot for the given security. Allow up to 6 digits.
	RoundLotSize string `csv:"Round Lot Size"`
	// Identifies whether the security is an exchange traded fund (ETF). Possible values:
	//    Y = Yes, security is an ETF
	//    N = No, security is not an ETF
	ETF string `csv:"ETF"`
	NextShares string `csv:"NextShares"`
	ParsedSymbol string
}

var goTemplate = template.Must(template.New("").Parse(`// Code generated by go generate; DO NOT EDIT.
// FILE AUTO
// {{ .Timestamp }}
// using data from
package {{.Package}}

const (
	{{range .Listings -}}  
		// {{.ParsedSymbol}} {{.SecurityName}}
	{{.ParsedSymbol}} = "{{.Symbol}}"
	{{end}}
)
`))

func deserializeCompanyListings(data []byte) ([]CompanyListing, error) {
	var r = strings.NewReplacer(
		".", "_",
		"-", "__",
		"+", "___",
		"$", "____",
		"=", "_____",
		"^", "______",
		"#", "_______",
	)
	output := make([]CompanyListing, 0)
	for nasdaqItem := range parse(bytes.NewReader(data)) {
		if strings.Contains(nasdaqItem.V.Symbol, "File Creation Time") {
			continue
		}
		if nasdaqItem.V.NasdaqSymbol != "" {
			nasdaqItem.V.Symbol = nasdaqItem.V.NasdaqSymbol
		}
		// Replace all pairs.
		nasdaqItem.V.ParsedSymbol = r.Replace(nasdaqItem.V.Symbol)
		if secNameSplit := strings.Split(nasdaqItem.V.SecurityName, "-"); len(secNameSplit) > 1 {
			nasdaqItem.V.SecurityName = secNameSplit[0]
		}
		if len(nasdaqItem.V.SecurityName) > maxSymbolNameLength {
			nasdaqItem.V.SecurityName = nasdaqItem.V.SecurityName[:maxSymbolNameLength]
		}
		if len(nasdaqItem.V.ParsedSymbol) < 1 {
			continue
		}
		output = append(output, nasdaqItem.V)
	}
	return output, nil
}

func main() {
	var (
		nasdaqListings []CompanyListing
		othersListings []CompanyListing
		err            error
		nasdaqlisted   []byte
		otherslisted   []byte
	)
	if nasdaqlisted, err = fetchFtpFile("nasdaqlisted.txt"); err != nil {
		panic(err)
	}
	if otherslisted, err = fetchFtpFile("otherlisted.txt"); err != nil {
		panic(err)
	}
	nasdaqListings, err = deserializeCompanyListings(nasdaqlisted)
	othersListings, err = deserializeCompanyListings(otherslisted)
	os.Mkdir("go", 0755)
	os.Mkdir("go/nasdaq", 0755)
	os.Mkdir("go/other", 0755)
	f, err := os.Create("go/nasdaq/nasdaq.go")
	if err != nil {
		panic(err)
	}
	f.Close()
	goTemplate.Execute(f, struct {
		Package string
		Timestamp time.Time
		Listings []CompanyListing
	}{
		Package: "nasdaq",
		Timestamp: time.Now(),
		Listings:  nasdaqListings,
	})
	f.Close()

	f, err = os.Create("go/other/other.go")
	goTemplate.Execute(f, struct {
		Package string
		Timestamp time.Time
		Listings []CompanyListing
	}{
		Package: "other",
		Timestamp: time.Now(),
		Listings:  othersListings,
	})
	f.Close()
}
