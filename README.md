# ticker-symbols

## About

Exported/generated NASDAQ symbols parsed from http://ftp.nasdaqtrader.com/.

## Setup

### Installation

If you want to include the Go generated code in your project:

```bash
$ go get -u gitlab.com/brokerage-api/ticker-symbols
```

```go
package main

import (
	"fmt"
	"gitlab.com/brokerage-api/ticker-symbols/go/nasdaq"
)

func main() {
	fmt.Printf(nasdaq.ACEV)
}
```

### Run Codegen Locally

```bash
$ git clone https://gitlab.com/brokerage-api/ticker-symbols.git
$ go get -u github.com/jlaffaye/ftp
$ go generate
```
