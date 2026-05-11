# Anchor Go Client

Lightweight Go client generated from the Anchor OpenAPI specification.

## Installation

```bash
go get github.com/nanostack-dev/anchor/clients/go@latest
```

## Usage

```go
package main

import (
    "log"

    nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

func main() {
    client, err := nanoclient.NewClientWithConfig(nanoclient.Config{
        BaseURL: "https://api.anchor.com",
        Token:   "your-api-token",
    })
    if err != nil {
        log.Fatal(err)
    }

    _ = client
}
```

## Development

Generate the client from the Anchor root:

```bash
make generate-client
```
