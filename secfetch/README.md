# SecFetch

A Go middleware for utilizing the Fetch Metadata Request Headers to protect against CSRF and other cross-site attacks.

## Overview

The `secfetch` package provides a middleware that examines the `Sec-Fetch-*` headers sent by modern browsers to enforce stricter security policies for your web applications.

These headers include:
- `Sec-Fetch-Site`: Indicates the relationship between request initiator and target (`same-origin`, `same-site`, `cross-site`, `none`)
- `Sec-Fetch-Mode`: Indicates the request's mode (`navigate`, `cors`, `no-cors`, `same-origin`, etc.)
- `Sec-Fetch-Dest`: Indicates the request's destination (`document`, `image`, `script`, etc.)
- `Sec-Fetch-User`: Indicates if the request was initiated by user action

## Features

- Blocks cross-site requests by default
- Enforces strict checks for form submissions
- Configurable through various options
- Simple API that matches Go standard http patterns
- Integration with github.com/lstoll/web package using HandleOpt

## Usage

Basic usage with default protections:

```go
import (
    "net/http"
    "github.com/lstoll/web/secfetch"
)

func main() {
    mux := http.NewServeMux()

    // Set up your routes
    mux.HandleFunc("/", homeHandler)

    // Wrap with secfetch protection
    protected := secfetch.Protect(mux)

    http.ListenAndServe(":8080", protected)
}
```

With custom options:

```go
// Allow cross-site navigation (like following links from other sites)
protected := secfetch.Protect(
    mux,
    secfetch.AllowCrossSiteNavigation{},
)

// Specify custom allowed request modes
protected := secfetch.Protect(
    mux,
    secfetch.WithAllowedModes("navigate", "same-origin", "cors"),
)

// Specify custom allowed request destinations
protected := secfetch.Protect(
    mux,
    secfetch.WithAllowedDests("document", "image", "style"),
)

// Combined options
protected := secfetch.Protect(
    mux,
    secfetch.AllowCrossSiteNavigation{},
    secfetch.WithAllowedModes("navigate", "same-origin", "cors"),
    secfetch.WithAllowedDests("document", "empty", "image"),
)
```

## Integration with web package

This package is designed to work seamlessly with the `github.com/lstoll/web` package. You can use it in two ways:

### Standard middleware

```go
import (
    "github.com/lstoll/web"
    "github.com/lstoll/web/secfetch"
)

func main() {
    // Create server config
    config := &web.Config{
        // ... other configuration
    }

    // Create server
    server, err := web.NewServer(config)
    if err != nil {
        // handle error
    }

    // Create your handler
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Handle request
    })

    // Apply protection and add to server
    server.HandleBrowser("/protected", secfetch.Protect(handler))
}
```

### Using HandleOpt system

The secfetch package also provides integration with the web package's HandleOpt system, allowing you to use the options directly with HandleBrowser:

```go
import (
    "github.com/lstoll/web"
    "github.com/lstoll/web/secfetch"
)

func main() {
    // Create server config
    config := &web.Config{
        // ... other configuration
    }

    // Create server
    server, err := web.NewServer(config)
    if err != nil {
        // handle error
    }

    // Register WebProtect as middleware
    server.HandleBrowser("/api", apiHandler, secfetch.WebProtect)

    // With options
    server.HandleBrowser("/crosssite", publicHandler,
        secfetch.WebProtect,
        secfetch.AllowNav{},  // Allow cross-site navigation
    )

    // With more options
    server.HandleBrowser("/api/public", publicAPIHandler,
        secfetch.WebProtect,
        secfetch.AllowAPI{},   // Allow cross-site API access
        secfetch.SecFetchOpt{  // Pass custom secfetch options
            Options: []secfetch.Option{
                secfetch.WithAllowedModes("navigate", "cors"),
                secfetch.WithAllowedDests("empty", "document"),
            },
        },
    )
}
```

## Browser Support

This middleware is designed for modern browsers that support Fetch Metadata headers. Older browsers that don't support these headers will have limited functionality - GET requests may be allowed, but POST and other methods will be rejected.

## References

- [Fetch Metadata Request Headers](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers#fetch_metadata_request_headers)
- [Protect your resources from web attacks with Fetch Metadata](https://web.dev/articles/fetch-metadata)
